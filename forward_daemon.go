package docker

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

var (
	readDeadline  = 30
	writeDeadline = 30
)

// ForwardSSHConfig for a cleaner version
type ForwardSSHConfig struct {
	Address        string
	User           string
	PrivateKeyFile string
	Password       string
}

// ForwardConfig the configuration for the forward
type ForwardConfig struct {
	JumpHostConfigs []*ForwardSSHConfig
	EndHostConfig   *ForwardSSHConfig
	LocalAddress    string
	RemoteAddress   string
}

// Forward wraps the forward
type Forward struct {
	quit      chan bool
	bootstrap chan bool
	config    *ForwardConfig
}

// NewForward creates new forward
func NewForward(config *ForwardConfig) (*Forward, error) {
	if err := checkConfig(config); err != nil {
		return nil, err
	}
	t := &Forward{
		quit:      make(chan bool),
		bootstrap: make(chan bool),
		config:    config,
	}
	go t.run()
	return t, nil
}

// checkConfig checks the config if it is feasible
func checkConfig(config *ForwardConfig) error {
	if config == nil {
		return fmt.Errorf("ForwardConfig cannot be nil")
	}

	if len(config.JumpHostConfigs) > 1 {
		return fmt.Errorf("Only 1 jump host is supported atm")
	}
	for _, jumpConfig := range config.JumpHostConfigs {
		if err := checkSSHConfig(jumpConfig); err != nil {
			return err
		}
	}
	if err := checkSSHConfig(config.EndHostConfig); err != nil {
		return err
	}
	if config.LocalAddress == "" || config.RemoteAddress == "" {
		return fmt.Errorf("LocalAddress and RemoteAddress have to be set")
	}

	return nil
}

// checkSSSConfig checks the ssh config for feasibility
func checkSSHConfig(sshConfig *ForwardSSHConfig) error {
	if sshConfig == nil {
		return fmt.Errorf("SSHConfig cannot be nil")
	}
	if sshConfig.User == "" {
		return fmt.Errorf("User cannot be empty")
	}
	if sshConfig.Address == "" {
		return fmt.Errorf("Address cannot be empty")
	}
	if sshConfig.PrivateKeyFile == "" && sshConfig.Password == "" {
		return fmt.Errorf("Either PrivateKeyFile or Password has to be set")
	}

	return nil
}

// convertToSSHConfig converts the given ssh config into
// and *ssh.ClienConfig while preferrring privateKeyFiles
func convertToSSHConfig(toConvert *ForwardSSHConfig) *ssh.ClientConfig {
	config := &ssh.ClientConfig{
		User:            toConvert.User,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	if toConvert.PrivateKeyFile != "" {
		config.Auth = []ssh.AuthMethod{publicKeyFile(toConvert.PrivateKeyFile)}
	} else {
		config.Auth = []ssh.AuthMethod{ssh.Password(toConvert.Password)}
	}
	return config
}

// buildSSSClient builds the *ssh.Client connection via the jump
// host to the end host. ATM only one jump host is supported
func (t *Forward) buildSSHClient() (*ssh.Client, error) {
	endHostConfig := convertToSSHConfig(t.config.EndHostConfig)
	if len(t.config.JumpHostConfigs) > 0 { //TODO ATM
		jumpHostConfig := convertToSSHConfig(t.config.JumpHostConfigs[0])
		log.Printf("[DEBUG] sshconf %+v", jumpHostConfig)

		log.Println("[DEBUG] local -> jump before dial")
		jumpHostClient, err := ssh.Dial("tcp", t.config.JumpHostConfigs[0].Address, jumpHostConfig)
		if err != nil {
			log.Printf("[DEBUG] in ssh.Dial to jump host error: %s", err)
			return nil, fmt.Errorf("ssh.Dial to jump host failed: %s", err)
		}
		log.Println("[DEBUG] local -> jump dialed")

		jumpHostConn, err := jumpHostClient.Dial("tcp", t.config.EndHostConfig.Address)
		if err != nil {
			return nil, fmt.Errorf("ssh.Dial from jump host to end server failed: %s", err)
		}
		log.Println("[DEBUG] jump -> endhost dialed")

		ncc, chans, reqs, err := ssh.NewClientConn(jumpHostConn, t.config.EndHostConfig.Address, endHostConfig)
		if err != nil {
			jumpHostConn.Close()
			return nil, fmt.Errorf("Failed to create ssh client to end host: %s", err)
		}
		log.Println("[DEBUG] jump -> endhost new client conn")
		finalClient := ssh.NewClient(ncc, chans, reqs)
		log.Println("[DEBUG] final client creation")

		return finalClient, nil
	}

	endHostClient, err := ssh.Dial("tcp", t.config.EndHostConfig.Address, endHostConfig)
	if err != nil {
		return nil, fmt.Errorf("ssh.Dial to jump host failed: %s", err)
	}

	return endHostClient, nil
}

func (t *Forward) buildLocalConnection(localListener net.Listener) (net.Conn, error) {
	localConn, err := localListener.Accept()
	if err != nil {
		return nil, fmt.Errorf("localListener.Accept failed: %v", err)
	}

	return localConn, nil
}

func (t *Forward) run() error {
	////////// === bootstrap
	sshClient, err := t.buildSSHClient()
	if err != nil {
		return fmt.Errorf("Error building SSH client: %s", err)
	}

	localListener, err := net.Listen("tcp", t.config.LocalAddress)
	if err != nil {
		return fmt.Errorf("net.Listen failed: %v", err)
	}
	log.Println("-> build local listener")
	defer func() {
		localListener.Close()
		log.Println("x-> local listener CLOSED")
	}()

	t.bootstrap <- true

	////////// === establish connection
	count := 1
	for {
		log.Printf("-> before built/accepted SSH remote connection: %d", count)
		endHostConn, err := sshClient.Dial("tcp", t.config.RemoteAddress)
		if err != nil {
			return fmt.Errorf("Failed to connect on end host to '%s': %s", t.config.RemoteAddress, err)
		}
		endHostConn.SetReadDeadline(time.Now().Add(time.Duration(readDeadline) * time.Second))
		endHostConn.SetWriteDeadline(time.Now().Add(time.Duration(writeDeadline) * time.Second))
		log.Printf("-> after accepted SSH remote connection: %d", count)
		defer func() {
			endHostConn.Close()
			log.Println("x-> remote connection CLOSED")
		}()

		log.Printf("-> before built/accepted local connection: %d", count)
		localConn, err := t.buildLocalConnection(localListener)
		if err != nil {
			return fmt.Errorf("Error building local connection: %s", err)
		}
		localConn.SetReadDeadline(time.Now().Add(time.Duration(readDeadline) * time.Second))
		localConn.SetWriteDeadline(time.Now().Add(time.Duration(writeDeadline) * time.Second))
		log.Println("-> accepted local connection")
		defer func() {
			localConn.Close()
			log.Println("x-> local connection CLOSED")
		}()

		if err != nil {
			log.Fatalf("listen.Accept failed: %v", err)
			select {
			case <-t.quit:
				log.Println("-> quitting")
				return nil
			default:
				log.Println("-> continueing")
				continue
			}
		} // if

		t.handleForward(localConn, endHostConn)
		count++
	} // for
}

func (t *Forward) handleForward(localConn, sshConn net.Conn) {
	log.Println("-> start handle forward")

	go func() {
		writtenToSSH, err := io.Copy(sshConn, localConn) // ssh <- local
		if err != nil {
			log.Fatalf("io.Copy 1 (written to SSH) failed: %v", err)
		}
		log.Printf("-+> written to SSH: %d", writtenToSSH)
	}()

	go func() {
		writtenToLocal, err := io.Copy(localConn, sshConn) // local <- ssh
		if err != nil {
			log.Fatalf("io.Copy 2 (written to local) failed: %v", err)
		}
		log.Printf("-+> written to local: %d", writtenToLocal)
	}()
}

// Stop stops the forward
func (t *Forward) Stop() {
	close(t.quit)
}

// publicKeyFile helper to read the key files
func publicKeyFile(file string) ssh.AuthMethod {
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
