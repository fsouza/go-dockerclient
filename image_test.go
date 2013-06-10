// Copyright 2013 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"encoding/json"
	"github.com/dotcloud/docker"
	"net/http"
	"reflect"
	"testing"
)

func TestListImages(t *testing.T) {
	body := `[
     {
             "Repository":"base",
             "Tag":"ubuntu-12.10",
             "Id":"b750fe79269d",
             "Created":1364102658
     },
     {
             "Repository":"base",
             "Tag":"ubuntu-quantal",
             "Id":"b750fe79269d",
             "Created":1364102658
     }
]`
	var expected []docker.APIImages
	err := json.Unmarshal([]byte(body), &expected)
	if err != nil {
		t.Fatal(err)
	}
	client := Client{
		endpoint: "http://localhost:4243",
		client: &http.Client{
			Transport: &FakeRoundTripper{message: body, status: http.StatusOK},
		},
	}
	images, err := client.ListImages(false)
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(images, expected) {
		t.Errorf("ListImages: Wrong return value. Want %#v. Got %#v.", expected, images)
	}
}

func TestListImagesParameters(t *testing.T) {
	fakeRT := FakeRoundTripper{message: "null", status: http.StatusOK}
	client := Client{
		endpoint: "http://localhost:4243",
		client:   &http.Client{Transport: &fakeRT},
	}
	_, err := client.ListImages(false)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	if req.Method != "GET" {
		t.Errorf("ListImages(false: Wrong HTTP method. Want GET. Got %s.", req.Method)
	}
	if all := req.URL.Query().Get("all"); all != "0" {
		t.Errorf("ListImages(false): Wrong parameter. Want all=0. Got all=%s", all)
	}
	fakeRT.Reset()
	_, err = client.ListImages(true)
	if err != nil {
		t.Fatal(err)
	}
	req = fakeRT.requests[0]
	if all := req.URL.Query().Get("all"); all != "1" {
		t.Errorf("ListImages(true): Wrong parameter. Want all=1. Got all=%s", all)
	}
}
