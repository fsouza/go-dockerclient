// Copyright 2016 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"encoding/json"
	"net/url"
	"strconv"

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

func (c *Client) SwarmJoin(opts swarm.JoinRequest) error {
	path := "/swarm/join"
	_, err := c.do("POST", path, doOptions{
		data:      opts,
		forceJSON: true,
	})
	return err
}

func (c *Client) SwarmLeave(force bool) error {
	params := make(url.Values)
	if force {
		params.Set("force", "1")
	}
	path := "/swarm/leave?" + params.Encode()
	_, err := c.do("POST", path, doOptions{})
	return err
}

type SwarmUpdateOptions struct {
	Version            int
	RotateWorkerToken  bool
	RotateManagerToken bool
	Swarm              swarm.Spec
}

func (c *Client) SwarmUpdate(opts SwarmUpdateOptions) error {
	params := make(url.Values)
	params.Set("version", strconv.Itoa(opts.Version))
	params.Set("rotateWorkerToken", strconv.FormatBool(opts.RotateWorkerToken))
	params.Set("rotateManagerToken", strconv.FormatBool(opts.RotateManagerToken))
	path := "/swarm/update?" + params.Encode()
	_, err := c.do("POST", path, doOptions{
		data:      opts.Swarm,
		forceJSON: true,
	})
	return err
}
