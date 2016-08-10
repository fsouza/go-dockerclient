// Copyright 2016 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"encoding/json"
	"net/url"
	"strconv"

	"golang.org/x/net/context"

	"github.com/docker/engine-api/types/swarm"
)

type SwarmInitOptions struct {
	swarm.InitRequest
	Context context.Context
}

// SwarmInit initializes a new Swarm and returns the node ID.
// See https://goo.gl/hzkgWu for more details.
func (c *Client) SwarmInit(opts SwarmInitOptions) (string, error) {
	path := "/swarm/init"
	resp, err := c.do("POST", path, doOptions{
		data:      opts.InitRequest,
		forceJSON: true,
		context:   opts.Context,
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

type SwarmJoinOptions struct {
	swarm.JoinRequest
	Context context.Context
}

// SwarmJoin joins an existing Swarm.
// See https://goo.gl/TdhJWU for more details.
func (c *Client) SwarmJoin(opts SwarmJoinOptions) error {
	path := "/swarm/join"
	_, err := c.do("POST", path, doOptions{
		data:      opts.JoinRequest,
		forceJSON: true,
		context:   opts.Context,
	})
	return err
}

type SwarmLeaveOptions struct {
	Force   bool
	Context context.Context
}

// SwarmLeave leaves a Swarm.
// See https://goo.gl/UWDlLg for more details.
func (c *Client) SwarmLeave(opts SwarmLeaveOptions) error {
	params := make(url.Values)
	if opts.Force {
		params.Set("force", "1")
	}
	path := "/swarm/leave?" + params.Encode()
	_, err := c.do("POST", path, doOptions{
		context: opts.Context,
	})
	return err
}

type SwarmUpdateOptions struct {
	Version            int
	RotateWorkerToken  bool
	RotateManagerToken bool
	Swarm              swarm.Spec
	Context            context.Context
}

// SwarmUpdate updates a Swarm.
// See https://goo.gl/vFbq36 for more details.
func (c *Client) SwarmUpdate(opts SwarmUpdateOptions) error {
	params := make(url.Values)
	params.Set("version", strconv.Itoa(opts.Version))
	params.Set("rotateWorkerToken", strconv.FormatBool(opts.RotateWorkerToken))
	params.Set("rotateManagerToken", strconv.FormatBool(opts.RotateManagerToken))
	path := "/swarm/update?" + params.Encode()
	_, err := c.do("POST", path, doOptions{
		data:      opts.Swarm,
		forceJSON: true,
		context:   opts.Context,
	})
	return err
}
