// Copyright 2015 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import "encoding/json"

// APIVolumes represents a volume.
//
// See https://goo.gl/FZA4BK for more details.
type APIVolumes struct {
	Name       string `json:"Name" yaml:"Name"`
	Driver     string `json:"Driver,omitempty" yaml:"Driver,omitempty"`
	Mountpoint string `json:"Mountpoint,omitempty" yaml:"Mountpoint,omitempty"`
}

// ListVolumesOptions specify parameters to the ListVolumes function.
//
// See https://goo.gl/FZA4BK for more details.
type ListVolumesOptions struct {
	Filters map[string][]string
}

// ListVolumes returns a list of available volumes in the server.
//
// See https://goo.gl/FZA4BK for more details.
func (c *Client) ListVolumes(opts ListVolumesOptions) ([]APIVolumes, error) {
	body, _, err := c.do("GET", "/volumes?"+queryString(opts), doOptions{})
	if err != nil {
		return nil, err
	}
	m := make(map[string]interface{})
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	var volumes []APIVolumes
	volumesJSON, ok := m["Volumes"]
	if !ok {
		return volumes, nil
	}
	data, err := json.Marshal(volumesJSON)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &volumes); err != nil {
		return nil, err
	}
	return volumes, nil
}
