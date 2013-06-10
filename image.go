// Copyright 2013 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"encoding/json"
	"errors"
	"github.com/dotcloud/docker"
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
