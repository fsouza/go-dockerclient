// Copyright 2016 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !windows

package docker

import (
	"context"
	"net"
	"net/http"
)

// initializeNativeClient initializes the native Unix domain socket client on
// Unix-style operating systems
func (c *Client) initializeNativeClient() {
	if c.endpointURL.Scheme != unixProtocol {
		return
	}
	tr := defaultTransport()
	c.initializeNativeClientTransport(tr)
}

func (c *Client) initializeNativeClientTransport(tr *http.Transport) {
	sockPath := c.endpointURL.Path
	tr.Dial = func(network, addr string) (net.Conn, error) {
		return c.Dialer.Dial(unixProtocol, sockPath)
	}
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return c.Dialer.Dial(unixProtocol, sockPath)
	}
	c.HTTPClient.Transport = tr
}
