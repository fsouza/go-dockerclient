// +build !windows
// Copyright 2016 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package docker provides a client for the Docker remote API.
//
// See https://goo.gl/G3plxW for more details on the remote API.
package docker

import (
	"fmt"
	"net"
	"net/http"

	"github.com/docker/docker/opts"
	"github.com/hashicorp/go-cleanhttp"
)

// DefaultDockerHost returns the default docker socket for the current OS
func DefaultDockerHost() (string, error) {
	// If we do not have a host, default to unix socket
	return opts.ValidateHost(fmt.Sprintf("unix://%s", opts.DefaultUnixSocket))
}

// initializeNativeClient initializes the native Unix domain socket client on
// Unix-style operating systems
func (c *Client) initializeNativeClient() {
	if c.endpointURL.Scheme != unixProtocol {
		return
	}
	socketPath := c.endpointURL.Path
	tr := cleanhttp.DefaultTransport()
	tr.Dial = func(network, addr string) (net.Conn, error) {
		return c.Dialer.Dial(unixProtocol, socketPath)
	}
	c.nativeHTTPClient = &http.Client{Transport: tr}
}
