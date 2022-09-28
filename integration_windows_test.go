// Copyright 2019 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build docker_integration

package docker

const integrationDockerImage = "mcr.microsoft.com/windows/servercore:ltsc2022"

func integrationCreateContainerOpts(imageName string, hostConfig *HostConfig) CreateContainerOptions {
	return CreateContainerOptions{
		Config: &Config{
			Image: imageName,
			Cmd:   []string{"powershell", "-Command", `Write-Host "hello hello"`},
		},
		HostConfig: hostConfig,
	}
}
