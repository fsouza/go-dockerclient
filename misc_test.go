// Copyright 2013 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"github.com/dotcloud/docker"
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

func TestVersion(t *testing.T) {
	body := `{
     "Version":"0.2.2",
     "GitCommit":"5a2a5cc+CHANGES",
     "GoVersion":"go1.0.3"
}`
	fakeRT := FakeRoundTripper{message: body, status: http.StatusOK}
	client := Client{
		endpoint: "http://localhost:4243/",
		client:   &http.Client{Transport: &fakeRT},
	}
	expected := docker.APIVersion{
		Version:   "0.2.2",
		GitCommit: "5a2a5cc+CHANGES",
		GoVersion: "go1.0.3",
	}
	version, err := client.Version()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*version, expected) {
		t.Errorf("Version(): Wrong result. Want %#v. Got %#v.", expected, version)
	}
	req := fakeRT.requests[0]
	if req.Method != "GET" {
		t.Errorf("Version(): wrong request method. Want GET. Got %s.", req.Method)
	}
	u, _ := url.Parse(client.getURL("/version"))
	if req.URL.Path != u.Path {
		t.Errorf("Version(): wrong request path. Want %q. Got %q.", u.Path, req.URL.Path)
	}
}

func TestVersionError(t *testing.T) {
	client := Client{
		endpoint: "http://localhost:4242",
		client: &http.Client{
			Transport: &FakeRoundTripper{message: "internal error", status: http.StatusInternalServerError},
		},
	}
	version, err := client.Version()
	if version != nil {
		t.Errorf("Version(): expected <nil> value, got %#v.", version)
	}
	if err == nil {
		t.Error("Version(): unexpected <nil> error")
	}
}
