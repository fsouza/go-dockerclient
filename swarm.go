// Copyright 2016 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"encoding/json"

	"github.com/docker/engine-api/types/swarm"
)

func (c *Client) SwarmInit(opts swarm.InitRequest) (string, error) {
	path := "/swarm/init"
	resp, err := c.do("POST", path, doOptions{
		data:      opts,
		forceJSON: true,
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var response string
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}
	return response, nil
}
