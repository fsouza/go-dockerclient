// Copyright 2013 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"bytes"
	"github.com/dotcloud/docker/engine"
	"io"
)

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
