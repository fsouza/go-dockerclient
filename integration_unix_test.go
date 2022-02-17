// Copyright 2019 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build docker_integration && !windows
// +build docker_integration,!windows

package docker

const integrationDockerImage = "alpine:latest"

func integrationCreateContainerOpts(imageName string, hostConfig *HostConfig) CreateContainerOptions {
	return CreateContainerOptions{
		Config: &Config{
			Image: imageName,
			Cmd:   []string{"sh", "-c", `echo "hello hello"`},
		},
		HostConfig: hostConfig,
	}
}
