// Copyright 2015 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build docker_integration

package docker

import (
	"bytes"
	"reflect"
	"strings"
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
	// split stdout by lines to make sure the test is the same on Windows
	// and Linux. Life is hard.
	expected := []string{
		"hello hello",
	}
	if stdoutLines := getLines(&stdout); !reflect.DeepEqual(stdoutLines, expected) {
		t.Errorf("Got wrong stdout from logs.\nWant:\n%#v.\n\nGot:\n%#v.", expected, stdoutLines)
	}
}

func getLines(buf *bytes.Buffer) []string {
	var lines []string
	for _, line := range strings.Split(buf.String(), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func pullImage(t *testing.T) string {
	imageName := integrationDockerImage
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
