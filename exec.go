// Copyright 2014 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Docs can currently be found at https://github.com/docker/docker/blob/master/docs/sources/reference/api/docker_remote_api_v1.15.md#exec-create

package docker

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// CreateExecOptions specify parameters to the CreateExecContainer function.
//
// TODO: Add link to docs once Docker 1.3 is out
type CreateExecOptions struct {
	Detach       bool     `json:"Detach,omitempty" yaml:"Detach,omitempty"`
	AttachStdin  bool     `json:"AttachStdin,omitempty" yaml:"AttachStdin,omitempty"`
	AttachStdout bool     `json:"AttachStdout,omitempty" yaml:"AttachStdout,omitempty"`
	AttachStderr bool     `json:"AttachStderr,omitempty" yaml:"AttachStderr,omitempty"`
	Tty          bool     `json:"Tty,omitempty" yaml:"Tty,omitempty"`
	Cmd          []string `json:"Cmd,omitempty" yaml:"Cmd,omitempty"`
	Container    string   `json:"Container,omitempty" yaml:"Container,omitempty"`
}

// StartExecOptions specify parameters to the StartExecContainer function.
//
// TODO: Add link to docs once Docker 1.3 is out
type StartExecOptions struct {
	Detach bool `json:"Detach,omitempty" yaml:"Detach,omitempty"`
	Tty    bool `json:"Tty,omitempty" yaml:"Tty,omitempty"`
}

type Exec struct {
	Id string `json:"Id,omitempty" yaml:"Id,omitempty"`
}

// CreateExec sets up an exec instance in a running container `id`, returning the exec
// instance, or an error in case of failure.
//
// TODO: Add link to docs once Docker 1.3 is out
func (c *Client) CreateExec(opts CreateExecOptions) (*Exec, error) {
	path := "/containers/" + opts.Container + "/exec"
	body, status, err := c.do("POST", path, opts)
	if status == http.StatusNotFound {
		return nil, &NoSuchContainer{ID: opts.Container}
	}
	if err != nil {
		return nil, err
	}
	var exec Exec
	err = json.Unmarshal(body, &exec)
	if err != nil {
		return nil, err
	}

	return &exec, nil
}

// TODO: Add link to docs once Docker 1.3 is out
func (c *Client) StartExec(id string, opts StartExecOptions) error {
	if id == "" {
		return &NoSuchExec{ID: id}
	}

	path := "/exec/" + id + "/start"
	_, status, err := c.do("POST", path, opts)
	if status == http.StatusNotFound {
		return &NoSuchExec{ID: id}
	}
	if err != nil {
		return err
	}
	return nil
}

// TODO: Add link to docs once Docker 1.3 is out
func (c *Client) ResizeExecTTY(id string, height, width int) error {
	params := make(url.Values)
	params.Set("h", strconv.Itoa(height))
	params.Set("w", strconv.Itoa(width))
	_, _, err := c.do("POST", "/exec/"+id+"/resize?"+params.Encode(), nil)
	return err
}

// NoSuchExec is the error returned when a given exec instance does not exist.
type NoSuchExec struct {
	ID string
}

func (err *NoSuchExec) Error() string {
	return "No such exec instance: " + err.ID
}
