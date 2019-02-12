// Copyright 2015 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build docker_integration

package docker

import (
	"bytes"
	"runtime"
	"testing"
)

func TestIntegrationPullCreateStartLogs(t *testing.T) {
	imageName := pullImage(t)
	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	hostConfig := HostConfig{PublishAllPorts: true}
	createOpts := integrationCreateContainerOpts(imageName, &hostConfig)
	container, err := client.CreateContainer(createOpts)
	if err != nil {
		t.Fatal(err)
	}
	err = client.StartContainer(container.ID, &hostConfig)
	if err != nil {
		t.Fatal(err)
	}
	status, err := client.WaitContainer(container.ID)
	if err != nil {
		t.Error(err)
	}
	if status != 0 {
		t.Errorf("WaitContainer(%q): wrong status. Want 0. Got %d", container.ID, status)
	}
	var stdout, stderr bytes.Buffer
	logsOpts := LogsOptions{
		Container:    container.ID,
		OutputStream: &stdout,
		ErrorStream:  &stderr,
		Stdout:       true,
		Stderr:       true,
	}
	err = client.Logs(logsOpts)
	if err != nil {
		t.Error(err)
	}
	if stderr.String() != "" {
		t.Errorf("Got unexpected stderr from logs: %q", stderr.String())
	}
	expected := `Welcome to reality, wake up and rejoice
Welcome to reality, you've made the right choice
Welcome to reality, and let them hear your voice, shout it out!
`
	if stdout.String() != expected {
		t.Errorf("Got wrong stdout from logs.\nWant:\n%#v.\n\nGot:\n%#v.", expected, stdout.String())
	}
}

func pullImage(t *testing.T) string {
	os := runtime.GOOS
	if os != "windows" {
		os = "linux"
	}
	imageName := "fsouza/go-dockerclient-integration:" + os
	var buf bytes.Buffer
	pullOpts := PullImageOptions{
		Repository:   imageName,
		OutputStream: &buf,
	}
	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	err = client.PullImage(pullOpts, AuthConfiguration{})
	if err != nil {
		t.Logf("Pull output: %s", buf.String())
		t.Fatal(err)
	}
	return imageName
}
