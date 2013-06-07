// Copyright 2013 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package docker provides a client for the Docker remote API.
//
// See http://goo.gl/mxyql for more details on the remote API.
package docker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dotcloud/docker"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

// ErrInvalidEndpoint is the error returned by NewClient when the given
// endpoint is invalid.
var ErrInvalidEndpoint = errors.New("Invalid endpoint")

// Client is the basic type of this package. It provides methods for
// interaction with the API.
type Client struct {
	endpoint string
	client   *http.Client
}

// NewClient returns a Client instance ready for communication with the
// given server endpoint.
func NewClient(endpoint string) (*Client, error) {
	if !isValid(endpoint) {
		return nil, ErrInvalidEndpoint
	}
	return &Client{endpoint: endpoint, client: http.DefaultClient}, nil
}

func (c *Client) do(method, path string, data interface{}) ([]byte, int, error) {
	var params io.Reader
	if data != nil {
		buf, err := json.Marshal(data)
		if err != nil {
			return nil, -1, err
		}
		params = bytes.NewBuffer(buf)
	}
	req, err := http.NewRequest(method, c.getURL(path), params)
	if err != nil {
		return nil, -1, err
	}
	req.Header.Set("User-Agent", "Docker-Client/"+docker.VERSION)
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	} else if method == "POST" {
		req.Header.Set("Content-Type", "plain/text")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return nil, -1, fmt.Errorf("Can't connect to docker daemon. Is 'docker -d' running on this host?")
		}
		return nil, -1, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, -1, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, resp.StatusCode, newAPIClientError(resp.StatusCode, body)
	}
	return body, resp.StatusCode, nil
}

func (c *Client) getURL(path string) string {
	return fmt.Sprintf("%s/v%f%s", strings.TrimRight(c.endpoint, "/"), docker.APIVERSION, path)
}

func queryString(opts interface{}) string {
	if opts == nil {
		return ""
	}
	value := reflect.ValueOf(opts)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return ""
	}
	items := url.Values(map[string][]string{})
	for i := 0; i < value.NumField(); i++ {
		field := value.Type().Field(i)
		key := field.Tag.Get("qs")
		if key == "" {
			key = strings.ToLower(field.Name)
		}
		v := value.Field(i)
		switch v.Kind() {
		case reflect.Bool:
			if v.Bool() {
				items.Add(key, "1")
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if v.Int() > 0 {
				items.Add(key, strconv.FormatInt(v.Int(), 10))
			}
		case reflect.Float32, reflect.Float64:
			if v.Float() > 0 {
				items.Add(key, strconv.FormatFloat(v.Float(), 'f', -1, 64))
			}
		case reflect.String:
			if v.String() != "" {
				items.Add(key, v.String())
			}
		}
	}
	return items.Encode()
}

type apiClientError struct {
	status  int
	message string
}

func newAPIClientError(status int, body []byte) *apiClientError {
	return &apiClientError{status: status, message: string(body)}
}

func (e *apiClientError) Error() string {
	return fmt.Sprintf("API error (%d): %s", e.status, e.message)
}

func isValid(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	_, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		if e, ok := err.(*net.AddrError); ok {
			return e.Err == "missing port in address"
		}
		return false
	}
	number, err := strconv.ParseInt(port, 10, 64)
	return err == nil && number > 0 && number < 65536
}
