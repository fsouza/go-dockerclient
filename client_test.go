// Copyright 2013 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestNewAPIClient(t *testing.T) {
	endpoint := "http://localhost:4243"
	client, err := NewClient(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if client.endpoint != endpoint {
		t.Errorf("Expected endpoint %s. Got %s.", endpoint, client.endpoint)
	}
	if client.client != http.DefaultClient {
		t.Errorf("Expected http.Client %#v. Got %#v.", http.DefaultClient, client.client)
	}
}

func TestNewClientInvalidEndpoint(t *testing.T) {
	cases := []string{
		"htp://localhost:3243", "http://localhost:a", "localhost:8080",
		"", "localhost", "http://localhost:8080:8383", "http://localhost:65536",
		"https://localhost:-20",
	}
	for _, c := range cases {
		client, err := NewClient(c)
		if client != nil {
			t.Errorf("Want <nil> client for invalid endpoint, got %#v.", client)
		}
		if !reflect.DeepEqual(err, ErrInvalidEndpoint) {
			t.Errorf("NewClient(%q): Got invalid error for invalid endpoint. Want %#v. Got %#v.", c, ErrInvalidEndpoint, err)
		}
	}
}

func TestGetURL(t *testing.T) {
	var tests = []struct {
		endpoint string
		path     string
		expected string
	}{
		{"http://localhost:4243/", "/", fmt.Sprintf("http://localhost:4243/v%f/", apiVersion)},
		{"http://localhost:4243", "/", fmt.Sprintf("http://localhost:4243/v%f/", apiVersion)},
		{"http://localhost:4243", "/containers/ps", fmt.Sprintf("http://localhost:4243/v%f/containers/ps", apiVersion)},
		{"http://localhost:4243/////", "/", fmt.Sprintf("http://localhost:4243/v%f/", apiVersion)},
	}
	var client Client
	for _, tt := range tests {
		client.endpoint = tt.endpoint
		got := client.getURL(tt.path)
		if got != tt.expected {
			t.Errorf("getURL(%q): Got %s. Want %s.", tt.path, got, tt.expected)
		}
	}
}

func TestError(t *testing.T) {
	err := newAPIClientError(400, []byte("bad parameter"))
	expected := apiClientError{status: 400, message: "bad parameter"}
	if !reflect.DeepEqual(expected, *err) {
		t.Errorf("Wrong error type. Want %#v. Got %#v.", expected, *err)
	}
	message := "API error (400): bad parameter"
	if err.Error() != message {
		t.Errorf("Wrong error message. Want %q. Got %q.", message, err.Error())
	}
}

func TestQueryString(t *testing.T) {
	v := float32(2.4)
	f32QueryString := fmt.Sprintf("w=%s&x=10&y=10.35", strconv.FormatFloat(float64(v), 'f', -1, 64))
	var tests = []struct {
		input interface{}
		want  string
	}{
		{&ListContainersOptions{All: true}, "all=1"},
		{ListContainersOptions{All: true}, "all=1"},
		{ListContainersOptions{Before: "something"}, "before=something"},
		{ListContainersOptions{Before: "something", Since: "other"}, "before=something&since=other"},
		{dumb{X: 10, Y: 10.35000}, "x=10&y=10.35"},
		{dumb{W: v, X: 10, Y: 10.35000}, f32QueryString},
		{dumb{X: 10, Y: 10.35000, Z: 10}, "x=10&y=10.35&zee=10"},
		{dumb{v: 4, X: 10, Y: 10.35000}, "x=10&y=10.35"},
		{nil, ""},
		{10, ""},
		{"not_a_struct", ""},
	}
	for _, tt := range tests {
		got := queryString(tt.input)
		if got != tt.want {
			t.Errorf("queryString(%v). Want %q. Got %q.", tt.input, tt.want, got)
		}
	}
}

type FakeRoundTripper struct {
	message  string
	status   int
	requests []*http.Request
}

func (rt *FakeRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	body := strings.NewReader(rt.message)
	rt.requests = append(rt.requests, r)
	return &http.Response{
		StatusCode: rt.status,
		Body:       ioutil.NopCloser(body),
	}, nil
}

func (rt *FakeRoundTripper) Reset() {
	rt.requests = nil
}

type dumb struct {
	v int
	W float32
	X int
	Y float64
	Z int `qs:"zee"`
}
