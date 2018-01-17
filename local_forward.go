package docker

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"

	"golang.org/x/crypto/ssh"
)

// SSHConfig for a cleaner version
type SSHConfig struct {
	address        string
	user           string
	privateKeyFile string
}

// Tunnel wraps the tunnel
type Tunnel struct {
	quit           chan bool
	endHostConfig  *SSHConfig
	jumpHostConfig *SSHConfig
	localAddress   string
	remoteAddress  string
}

// NewTunnel creates new tunnel to the endhost via the given jump host
func NewTunnel(endHostConfig, jumpHostConfig *SSHConfig, localAddress, remoteAddress string) *Tunnel {
	t := &Tunnel{
		quit:           make(chan bool),
		endHostConfig:  endHostConfig,
		jumpHostConfig: jumpHostConfig,
		localAddress:   localAddress,
		remoteAddress:  remoteAddress,
	}
	go t.run()
	return t
}

func (t *Tunnel) run() error {
	sshConn, err := t.buildSSHConnection()
	if err != nil {
		return fmt.Errorf("Error building SSH connection: %s", err)
	}
	defer sshConn.Close()
	log.Println("-> build ssh connection")

	localListener, err := net.Listen("tcp", t.localAddress)
	if err != nil {
		return fmt.Errorf("net.Listen failed: %v", err)
	}
	defer localListener.Close()
	log.Println("-> build local listener")

	for {
		log.Println("-> before accepted local connection")
		localConn, err := localListener.Accept()
		log.Println("-> accepted local connection")
		if err != nil {
			log.Fatalf("listen.Accept failed: %v", err)
			select {
			case <-t.quit:
				return nil
			default:
			}
			continue
		}

		go t.forward(localConn, sshConn)
	}
}

func (t *Tunnel) forward(localConn, sshConn net.Conn) error {
	// defer func() {
	// 	localConn.Close()
	// 	sshConn.Close()
	// }()

	log.Println("-> start forward")

	// Copy localConn.Reader to sshConn.Writer
	go func() {
		_, err := io.Copy(sshConn, localConn)
		if err != nil {
			log.Fatalf("io.Copy 1 failed: %v", err)
		}
	}()

	// Copy sshConn.Reader to localConn.Writer
	go func() {
		_, err := io.Copy(localConn, sshConn)
		if err != nil {
			log.Fatalf("io.Copy 2 failed: %v", err)
		}
	}()

	return nil
}

// Stop stops the tunnel
func (t *Tunnel) Stop() {
	close(t.quit)
}

// PublicKeyFile helper to read the key files
func PublicKeyFile(file string) ssh.AuthMethod {
	buffer, err := ioutil.ReadFile(file)
	if err != nil {
		return nil
	}

	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil
	}
	return ssh.PublicKeys(key)
}

func convertToSSHConfig(toConvert *SSHConfig) *ssh.ClientConfig {
	return &ssh.ClientConfig{
		User:            toConvert.user,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth: []ssh.AuthMethod{
			PublicKeyFile(toConvert.privateKeyFile),
		},
	}
}

func (t *Tunnel) buildSSHConnection() (net.Conn, error) {
	jumpHostConfig := convertToSSHConfig(t.jumpHostConfig)
	config := convertToSSHConfig(t.endHostConfig)

	jumpHostClient, err := ssh.Dial("tcp", t.jumpHostConfig.address, jumpHostConfig)
	if err != nil {
		return nil, fmt.Errorf("ssh.Dial to jump host failed: %s", err)
	}

	conn, err := jumpHostClient.Dial("tcp", t.endHostConfig.address)
	if err != nil {
		return nil, fmt.Errorf("ssh.Dial from jump host to end server failed: %s", err)
	}

	ncc, chans, reqs, err := ssh.NewClientConn(conn, t.endHostConfig.address, config)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("Failed to create ssh client to end host: %s", err)
	}

	sClient := ssh.NewClient(ncc, chans, reqs)

	sshConn, err := sClient.Dial("tcp", t.remoteAddress)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("Failed to connect on end host to '%s': %s", t.remoteAddress, err)
	}

	return sshConn, nil
}
