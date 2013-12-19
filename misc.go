// Copyright 2013 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"bytes"
	"encoding/json"
	"github.com/dotcloud/docker/engine"
	"io"
)

type APIInfo struct {
	Containers     int64
	Images         int64
	Debug          bool
	NFd            int64
	NGoroutines    int64
	MemoryLimit    bool
	SwapLimit      bool
	IPv4Forwarding bool
}

// Version returns version information about the docker server.
//
// See http://goo.gl/IqKNRE for more details.
func (c *Client) Version() (*engine.Env, error) {
	body, _, err := c.do("GET", "/version", nil)
	if err != nil {
		return nil, err
	}
	out := engine.NewOutput()
	remoteVersion, err := out.AddEnv()
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(out, bytes.NewReader(body)); err != nil {
		return nil, err
	}
	return remoteVersion, nil
}

// Info returns system-wide information, like the number of running containers.
//
// See http://goo.gl/LOmySw for more details.
func (c *Client) Info() (*APIInfo, error) {
	body, _, err := c.do("GET", "/info", nil)
	if err != nil {
		return nil, err
	}
	var info APIInfo
	err = json.Unmarshal(body, &info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}
