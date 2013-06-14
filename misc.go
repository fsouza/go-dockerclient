// Copyright 2013 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"encoding/json"
	"github.com/dotcloud/docker"
)

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
