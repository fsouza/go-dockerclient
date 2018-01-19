// Copyright 2018 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"strings"
	"testing"
)

func TestForwardConfigNotSet(t *testing.T) {
	t.Parallel()
	_, _, err := NewForward(nil)
	if err == nil {
		t.Errorf("Expected an error for a nil ForwardConfig but got none")
	}
	checkErrorContains(t, err, "ForwardConfig")
}
func TestForwardConfigTooManyJumpHosts(t *testing.T) {
	t.Parallel()
	forwardConfig := &ForwardConfig{
		JumpHostConfigs: []*ForwardSSHConfig{
			&ForwardSSHConfig{
				Address:        "10.0.0.1:22",
				User:           "jumpuser1",
				PrivateKeyFile: "/Users/abc/.ssh/id_rsa_jump_host1",
				Password:       "",
			},
			&ForwardSSHConfig{
				Address:        "10.0.0.2:22",
				User:           "jumpuser2",
				PrivateKeyFile: "/Users/abc/.ssh/id_rsa_jump_host2",
				Password:       "",
			},
		},
		EndHostConfig: &ForwardSSHConfig{
			Address:        "20.0.0.1:22",
			User:           "endhostuser",
			PrivateKeyFile: "/Users/abc/.ssh/id_rsa_end_host",
			Password:       "",
		},
		LocalAddress:  "localhost:2376",
		RemoteAddress: "localhost:2376",
	}

	_, _, err := NewForward(forwardConfig)
	if err == nil {
		t.Errorf("Expected an error for a too many jump hosts in ForwardConfig but got none")
	}
	checkErrorContains(t, err, "Only 1 jump host")
}
func TestForwardConfigInvalidJumpHostSSHConfig(t *testing.T) {
	t.Parallel()

	forwardConfig := createForwardConfig("10.0.0.1:22", "", "/Users/abc/.ssh/id_rsa_jump_host1", "")
	_, _, err := NewForward(forwardConfig)
	checkErrorContains(t, err, "User cannot be empty")

	forwardConfig = createForwardConfig("", "jumpuser", "/Users/abc/.ssh/id_rsa_jump_host1", "")
	_, _, err = NewForward(forwardConfig)
	checkErrorContains(t, err, "Address cannot be empty")

	forwardConfig = createForwardConfig("10.0.0.1:22", "jumpuser", "", "")
	_, _, err = NewForward(forwardConfig)
	checkErrorContains(t, err, "Either PrivateKeyFile or Password")

	forwardConfig = createForwardConfigWithAddresses("10.0.0.1:22", "jumpuser", "/Users/abc/.ssh/id_rsa_jump_host1", "", "localhost:2376", "")
	_, _, err = NewForward(forwardConfig)
	checkErrorContains(t, err, "LocalAddress and RemoteAddress have to be set")

	forwardConfig = createForwardConfigWithAddresses("10.0.0.1:22", "jumpuser", "", "mypwd", "", "localhost:2376")
	_, _, err = NewForward(forwardConfig)
	checkErrorContains(t, err, "LocalAddress and RemoteAddress have to be set")
}

//////////////
// Helpers
//////////////
func checkErrorContains(t *testing.T, err error, errorMsgtoContain string) {
	if !strings.Contains(err.Error(), errorMsgtoContain) {
		t.Errorf("Expected error to contain \n'%s' but got \n'%s'", errorMsgtoContain, err.Error())
	}
}

func createForwardConfig(jumpHostAddress, jumpHostUser, jumpHostPrivateKeyFile, jumpHostPassword string) *ForwardConfig {
	return createForwardConfigBase(jumpHostAddress, jumpHostUser, jumpHostPrivateKeyFile, jumpHostPassword, "localhost:2376", "localhost:2376")
}

func createForwardConfigWithAddresses(jumpHostAddress, jumpHostUser, jumpHostPrivateKeyFile, jumpHostPassword, localAddress, remoteAddress string) *ForwardConfig {
	return createForwardConfigBase(jumpHostAddress, jumpHostUser, jumpHostPrivateKeyFile, jumpHostPassword, localAddress, remoteAddress)
}

func createForwardConfigBase(jumpHostAddress, jumpHostUser, jumpHostPrivateKeyFile, jumpHostPassword, localAddress, remoteAddress string) *ForwardConfig {
	return &ForwardConfig{
		JumpHostConfigs: []*ForwardSSHConfig{
			&ForwardSSHConfig{
				Address:        jumpHostAddress,
				User:           jumpHostUser,
				PrivateKeyFile: jumpHostPrivateKeyFile,
				Password:       jumpHostPassword,
			},
		},
		EndHostConfig: &ForwardSSHConfig{
			Address:        "20.0.0.1:22",
			User:           "endhostuser",
			PrivateKeyFile: "/Users/abc/.ssh/id_rsa_end_host",
			Password:       "",
		},
		LocalAddress:  localAddress,
		RemoteAddress: remoteAddress,
	}
}
