// Copyright 2019 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build docker_integration && !windows

package docker

import (
	"testing"
)

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

func getExecID(t *testing.T, client *Client, containerID string, cmd []string) string {
	t.Helper()
	e, err := client.CreateExec(CreateExecOptions{
		Cmd:          cmd,
		Container:    containerID,
		AttachStdout: false,
		AttachStderr: false,
	})
	if err != nil {
		t.Fatalf("unexpected error creating exec: %s", err)
	}
	return e.ID
}

func verifyExitCode(t *testing.T, client *Client, execID string, expectExitCode int) {
	t.Helper()
	err := client.StartExec(execID, StartExecOptions{
		OutputStream: nil,
		ErrorStream:  nil,
		Detach:       false,
	})
	if err != nil {
		t.Fatalf("failure to start exec: %s", err)
	}

	ei, err := client.InspectExec(execID)
	if err != nil {
		t.Fatalf("failure to inspect exec: %s", err)
	}
	if ei.ExitCode != expectExitCode {
		t.Fatalf("incorrect exit code - want:(%d) got: (%d)", expectExitCode, ei.ExitCode)
	}
}

func TestIntegrationExecBlockingWait(t *testing.T) {
	imageName := pullImage(t)
	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	hostConfig := HostConfig{AutoRemove: true}
	createOpts := integrationCreateContainerOpts(imageName, &hostConfig)
	// keep the container running while we test exec
	createOpts.Config.Cmd = []string{"tail", "-f", "/dev/null"}

	container, err := client.CreateContainer(createOpts)
	if err != nil {
		t.Fatal(err)
	}
	err = client.StartContainer(container.ID, &hostConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// No need to shut down tail -f /dev/null gracefully ;)
		err := client.KillContainer(KillContainerOptions{
			ID: container.ID,
		})
		if err != nil {
			// Fail if kill somehow fails.
			t.Fatalf("failed to kill: %s", err)
		}

	})

	falseExec := getExecID(t, client, container.ID, []string{"/bin/false"})
	verifyExitCode(t, client, falseExec, 1)
	shellSleepExec := getExecID(t, client, container.ID, []string{"/bin/sh", "-c", "/bin/sleep 2 && exit 42"})
	verifyExitCode(t, client, shellSleepExec, 42)
}
