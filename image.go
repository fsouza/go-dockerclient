// Copyright 2013 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"encoding/json"
	"errors"
	"github.com/dotcloud/docker"
	"io"
	"net/http"
)

// Error returned when the image does not exist.
var ErrNoSuchImage = errors.New("No such image")

// ListImages returns the list of available images in the server.
//
// See http://goo.gl/5ZfHk for more details.
func (c *Client) ListImages(all bool) ([]docker.APIImages, error) {
	path := "/images/json?all="
	if all {
		path += "1"
	} else {
		path += "0"
	}
	body, _, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var images []docker.APIImages
	err = json.Unmarshal(body, &images)
	if err != nil {
		return nil, err
	}
	return images, nil
}

// RemoveImage removes a image by its name or ID.
//
// See http://goo.gl/J2FNF for more details.
func (c *Client) RemoveImage(name string) error {
	_, status, err := c.do("DELETE", "/images/"+name, nil)
	if status == http.StatusNotFound {
		return ErrNoSuchImage
	}
	return err
}

// InspectImage returns an image by its name or ID.
//
// See http://goo.gl/dqGQO for more details.
func (c *Client) InspectImage(name string) (*docker.Image, error) {
	body, status, err := c.do("GET", "/images/"+name+"/json", nil)
	if status == http.StatusNotFound {
		return nil, ErrNoSuchImage
	}
	if err != nil {
		return nil, err
	}
	var image docker.Image
	err = json.Unmarshal(body, &image)
	if err != nil {
		return nil, err
	}
	return &image, nil
}

// PushImageOptions options to use in the PushImage method.
type PushImageOptions struct {
	// Name or ID of the image
	Name string

	// Registry server to push the image
	Registry string
}

// PushImage pushes a image to a remote registry, logging progress to w.
//
// See http://goo.gl/Hx3CB for more details.
func (c *Client) PushImage(opts *PushImageOptions, w io.Writer) error {
	if opts == nil || opts.Name == "" {
		return ErrNoSuchImage
	}
	copy := PushImageOptions{Registry: opts.Registry}
	path := "/images/" + opts.Name + "/push?" + queryString(&copy)
	return c.stream("POST", path, nil, w)
}
