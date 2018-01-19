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
	readDeadline      = 30
	writeDeadline     = 30
	connectionTimeout = 8 * time.Second
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
	quit          chan bool
	config        *ForwardConfig
	forwardErrors chan error
}

// NewForward creates new forward
// 1: it build the SSH tunnel via the optional jump hosts
// 2: if this was successful a forward from the 'LocalAddress' to the 'RemoteAddress'
//    will be established
func NewForward(config *ForwardConfig) (*Forward, chan error, error) {
	/// == bootstrap
	if err := checkConfig(config); err != nil {
		return nil, nil, err
	}
	forwardErrors := make(chan error)
	t := &Forward{
		quit:          make(chan bool),
		config:        config,
		forwardErrors: forwardErrors,
	}
	sshClient, localListener, err := t.bootstrap()
	if err != nil {
		return nil, nil, err
	}

	/// == run the forward
	go t.run(sshClient, localListener)
	return t, forwardErrors, nil
}

// checkConfig checks the config if it is feasible
func checkConfig(config *ForwardConfig) error {
	if config == nil {
		return fmt.Errorf("ForwardConfig cannot be nil")
	}

	if len(config.JumpHostConfigs) > 1 { //TODO atm only one jump host is supported
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
		Timeout:         connectionTimeout,
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
func (f *Forward) buildSSHClient() (*ssh.Client, error) {
	endHostConfig := convertToSSHConfig(f.config.EndHostConfig)
	if len(f.config.JumpHostConfigs) > 0 { //TODO atm only one jump host is supported
		jumpHostConfig := convertToSSHConfig(f.config.JumpHostConfigs[0])
		jumpHostClient, err := ssh.Dial("tcp", f.config.JumpHostConfigs[0].Address, jumpHostConfig)
		if err != nil {
			return nil, fmt.Errorf("ssh.Dial to jump host failed: %s", err)
		}

		jumpHostConn, err := f.dialNextJump(jumpHostClient, f.config.EndHostConfig.Address)
		if err != nil {
			return nil, fmt.Errorf("ssh.Dial from jump to jump host failed: %s", err)
		}

		ncc, chans, reqs, err := ssh.NewClientConn(jumpHostConn, f.config.EndHostConfig.Address, endHostConfig)
		if err != nil {
			jumpHostConn.Close()
			return nil, fmt.Errorf("Failed to create ssh client to end host: %s", err)
		}

		return ssh.NewClient(ncc, chans, reqs), nil
	}

	endHostClient, err := ssh.Dial("tcp", f.config.EndHostConfig.Address, endHostConfig)
	if err != nil {
		return nil, fmt.Errorf("ssh.Dial directly to end host failed: %s", err)
	}

	return endHostClient, nil
}

func (f *Forward) dialNextJump(jumpHostClient *ssh.Client, nextJumpAddress string) (net.Conn, error) {
	// NOTE: no timeout param in ssh.Dial: https://github.com/golang/go/issues/20288
	// implement it by hand
	var jumpHostConn net.Conn
	connChan := make(chan net.Conn)
	go func() {
		jumpHostConn, err := jumpHostClient.Dial("tcp", nextJumpAddress)
		if err != nil {
			f.forwardErrors <- fmt.Errorf("ssh.Dial from jump host to end server failed: %s", err)
			return
		}
		connChan <- jumpHostConn
	}()
	select {
	case jumpHostConnSel := <-connChan:
		jumpHostConn = jumpHostConnSel
	case <-time.After(connectionTimeout):
		return nil, fmt.Errorf("ssh.Dial from jump host to next jump failed after timeout")
	}

	return jumpHostConn, nil
}

func (f *Forward) buildLocalConnection(localListener net.Listener) (net.Conn, error) {
	localConn, err := localListener.Accept()
	if err != nil {
		return nil, fmt.Errorf("Listen to local address failed: %v", err)
	}

	return localConn, nil
}

func (f *Forward) bootstrap() (*ssh.Client, net.Listener, error) {
	sshClient, err := f.buildSSHClient()
	if err != nil {
		return nil, nil, fmt.Errorf("Error building SSH client: %s", err)
	}

	localListener, err := net.Listen("tcp", f.config.LocalAddress)
	if err != nil {
		return nil, nil, fmt.Errorf("net.Listen failed: %v", err)
	}

	return sshClient, localListener, nil
}

func (f *Forward) run(sshClient *ssh.Client, localListener net.Listener) {
	defer func() {
		localListener.Close()
		sshClient.Close()
	}()

	jumpCount := 1
	for {
		endHostConn, err := sshClient.Dial("tcp", f.config.RemoteAddress) // TODO timeout here?
		if err != nil {
			f.forwardErrors <- fmt.Errorf("Failed to connect on end host to docker daemon at '%s': %s", f.config.RemoteAddress, err)
			return
		}
		endHostConn.SetReadDeadline(time.Now().Add(time.Duration(readDeadline) * time.Second))
		endHostConn.SetWriteDeadline(time.Now().Add(time.Duration(writeDeadline) * time.Second))
		defer endHostConn.Close()

		localConn, err := f.buildLocalConnection(localListener)
		if err != nil {
			f.forwardErrors <- fmt.Errorf("Error building local connection: %s", err)
			return
		}
		localConn.SetReadDeadline(time.Now().Add(time.Duration(readDeadline) * time.Second))
		localConn.SetWriteDeadline(time.Now().Add(time.Duration(writeDeadline) * time.Second))
		defer localConn.Close()

		if err != nil {
			select {
			case <-f.quit:
				return
			default:
				continue
			}
		}

		f.handleForward(localConn, endHostConn)
		jumpCount++
	}
}

func (f *Forward) handleForward(localConn, sshConn net.Conn) {
	go func() {
		_, err := io.Copy(sshConn, localConn) // ssh <- local
		if err != nil {
			log.Fatalf("io.Copy from localAddress -> remoteAddress failed: %v", err)
		}
	}()

	go func() {
		_, err := io.Copy(localConn, sshConn) // local <- ssh
		if err != nil {
			log.Fatalf("io.Copy from remoteAddress -> localAddress failed: %v", err)
		}
	}()
}

// Stop stops the forward
func (f *Forward) Stop() {
	close(f.quit)
	close(f.forwardErrors)
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
