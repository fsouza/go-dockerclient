// Copyright 2016 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !windows

package docker

import (
	"bytes"
	"context"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"path/filepath"
	"strings"

	"github.com/xanzy/ssh-agent"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// initializeNativeClient initializes the native Unix domain socket client on
// Unix-style operating systems
func (c *Client) initializeNativeClient() {
	if c.endpointURL.Scheme != unixProtocol {
		return
	}

	var jumpHostClient *ssh.Client
	// Connect via jump host if it is set
	if c.jHAddr != "" {
		// == 1:
		sshAgent, err := c.connectToAgent()
		if err != nil {
			log.Printf("error connecting to agent: %s", err)
			// return nil, err TODO
		}

		// == 2:
		jumpHostConf, err := buildSSHClientConfig(sshClientConfigOpts{
			user:       c.jHUser,
			privateKey: c.jHPrivateKey,
			password:   c.jHPassword,
			sshAgent:   sshAgent,
		})
		if err != nil {
			log.Printf("error building ssh config: %s", err)
			// return nil, err
		}
		// == 3:
		jumpHost := fmt.Sprintf("%s:%d", c.jHAddr, 22) // TODO
		// connectFunc := BastionConnectFunc("tcp", jHostHost, jHostConf, "tcp", host)
		log.Printf("[DEBUG] Connecting to jHost: %s", jumpHost)
		jumpHostClient, err = ssh.Dial("tcp", jumpHost, jumpHostConf) // TODO
		if err != nil {
			log.Printf("Error connecting to jHost (%s): %s", jumpHost, err)
		}
	}

	// set up the connection
	socketPath := c.endpointURL.Path
	tr := defaultTransport()
	// Deprecated! TODO
	// tr.Dial = func(network, addr string) (net.Conn, error) {
	// 	return c.Dialer.Dial(unixProtocol, socketPath)
	// }
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if jumpHostClient != nil {
			return jumpHostClient.Dial(unixProtocol, socketPath)
		}
		return c.Dialer.Dial(unixProtocol, socketPath)
	}
	c.HTTPClient.Transport = tr
}

func (c *Client) connectToAgent() (*sshAgent, error) {
	if c.Agent != true {
		// No agent configured
		return nil, nil
	}

	agent, conn, err := sshagent.New()
	if err != nil {
		return nil, err
	}

	// connection close is handled over in Communicator
	return &sshAgent{
		agent: agent,
		conn:  conn,
		id:    c.AgentIdentity,
	}, nil

}

// A tiny wrapper around an agent.Agent to expose the ability to close its
// associated connection on request.
type sshAgent struct {
	agent agent.Agent
	conn  net.Conn
	id    string
}

// TODO
type sshClientConfigOpts struct {
	privateKey string
	password   string
	sshAgent   *sshAgent
	user       string
}

// TODO
func buildSSHClientConfig(opts sshClientConfigOpts) (*ssh.ClientConfig, error) {
	conf := &ssh.ClientConfig{
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		User:            opts.user,
	}

	if opts.privateKey != "" {
		pubKeyAuth, err := readPrivateKey(opts.privateKey)
		if err != nil {
			return nil, err
		}
		conf.Auth = append(conf.Auth, pubKeyAuth)
	}

	if opts.password != "" {
		conf.Auth = append(conf.Auth, ssh.Password(opts.password))
	}

	if opts.sshAgent != nil {
		conf.Auth = append(conf.Auth, opts.sshAgent.Auth())
	}

	return conf, nil
}

func readPrivateKey(pk string) (ssh.AuthMethod, error) {
	// We parse the private key on our own first so that we can
	// show a nicer error if the private key has a password.
	block, _ := pem.Decode([]byte(pk))
	if block == nil {
		return nil, fmt.Errorf("Failed to read key %q: no key found", pk)
	}
	if block.Headers["Proc-Type"] == "4,ENCRYPTED" {
		return nil, fmt.Errorf(
			"Failed to read key %q: password protected keys are\n"+
				"not supported. Please decrypt the key prior to use.", pk)
	}

	signer, err := ssh.ParsePrivateKey([]byte(pk))
	if err != nil {
		return nil, fmt.Errorf("Failed to parse key file %q: %s", pk, err)
	}

	return ssh.PublicKeys(signer), nil
}

// make an attempt to either read the identity file or find a corresponding
// public key file using the typical openssh naming convention.
// This returns the public key in wire format, or nil when a key is not found.
func findIDPublicKey(id string) []byte {
	for _, d := range idKeyData(id) {
		signer, err := ssh.ParsePrivateKey(d)
		if err == nil {
			log.Println("[DEBUG] parsed id private key")
			pk := signer.PublicKey()
			return pk.Marshal()
		}

		// try it as a publicKey
		pk, err := ssh.ParsePublicKey(d)
		if err == nil {
			log.Println("[DEBUG] parsed id public key")
			return pk.Marshal()
		}

		// finally try it as an authorized key
		pk, _, _, _, err = ssh.ParseAuthorizedKey(d)
		if err == nil {
			log.Println("[DEBUG] parsed id authorized key")
			return pk.Marshal()
		}
	}

	return nil
}

// Try to read an id file using the id as the file path. Also read the .pub
// file if it exists, as the id file may be encrypted. Return only the file
// data read. We don't need to know what data came from which path, as we will
// try parsing each as a private key, a public key and an authorized key
// regardless.
func idKeyData(id string) [][]byte {
	idPath, err := filepath.Abs(id)
	if err != nil {
		return nil
	}

	var fileData [][]byte

	paths := []string{idPath}

	if !strings.HasSuffix(idPath, ".pub") {
		paths = append(paths, idPath+".pub")
	}

	for _, p := range paths {
		d, err := ioutil.ReadFile(p)
		if err != nil {
			log.Printf("[DEBUG] error reading %q: %s", p, err)
			continue
		}
		log.Printf("[DEBUG] found identity data at %q", p)
		fileData = append(fileData, d)
	}

	return fileData
}

// sortSigners moves a signer with an agent comment field matching the
// agent_identity to the head of the list when attempting authentication. This
// helps when there are more keys loaded in an agent than the host will allow
// attempts.
func (s *sshAgent) sortSigners(signers []ssh.Signer) {
	if s.id == "" || len(signers) < 2 {
		return
	}

	// if we can locate the public key, either by extracting it from the id or
	// locating the .pub file, then we can more easily determine an exact match
	idPk := findIDPublicKey(s.id)

	// if we have a signer with a connect field that matches the id, send that
	// first, otherwise put close matches at the front of the list.
	head := 0
	for i := range signers {
		pk := signers[i].PublicKey()
		k, ok := pk.(*agent.Key)
		if !ok {
			continue
		}

		// check for an exact match first
		if bytes.Equal(pk.Marshal(), idPk) || s.id == k.Comment {
			signers[0], signers[i] = signers[i], signers[0]
			break
		}

		// no exact match yet, move it to the front if it's close. The agent
		// may have loaded as a full filepath, while the config refers to it by
		// filename only.
		if strings.HasSuffix(k.Comment, s.id) {
			signers[head], signers[i] = signers[i], signers[head]
			head++
			continue
		}
	}

	ss := []string{}
	for _, signer := range signers {
		pk := signer.PublicKey()
		k := pk.(*agent.Key)
		ss = append(ss, k.Comment)
	}
}

func (s *sshAgent) Signers() ([]ssh.Signer, error) {
	signers, err := s.agent.Signers()
	if err != nil {
		return nil, err
	}

	s.sortSigners(signers)
	return signers, nil
}

func (s *sshAgent) Auth() ssh.AuthMethod {
	return ssh.PublicKeysCallback(s.Signers)
}
