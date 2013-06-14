// Copyright 2013 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"encoding/json"
	"github.com/dotcloud/docker"
)

// Version returns version information about the docker server.
func (c *Client) Version() (*docker.APIVersion, error) {
	body, _, err := c.do("GET", "/version", nil)
	if err != nil {
		return nil, err
	}
	var version docker.APIVersion
	err = json.Unmarshal(body, &version)
	if err != nil {
		return nil, err
	}
	return &version, nil
}

// Info returns system-wide information, like the number of running containers.
//
// See http://goo.gl/9eYZF for more details.
func (c *Client) Info() (*docker.APIInfo, error) {
	body, _, err := c.do("GET", "/info", nil)
	if err != nil {
		return nil, err
	}
	var info docker.APIInfo
	err = json.Unmarshal(body, &info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}
