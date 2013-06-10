// Copyright 2013 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"bytes"
	"encoding/json"
	"github.com/dotcloud/docker"
	"net/http"
	"net/url"
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

func TestRemoveImage(t *testing.T) {
	name := "test"
	fakeRT := FakeRoundTripper{message: "", status: http.StatusNoContent}
	client := Client{endpoint: "http://localhost:4243", client: &http.Client{Transport: &fakeRT}}
	err := client.RemoveImage(name)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	expectedMethod := "DELETE"
	if req.Method != expectedMethod {
		t.Errorf("RemoveImage(%q): Wrong HTTP method. Want %s. Got %s.", name, expectedMethod, req.Method)
	}
	u, _ := url.Parse(client.getURL("/images/" + name))
	if req.URL.Path != u.Path {
		t.Errorf("RemoveImage(%q): Wrong request path. Want %q. Got %q.", name, u.Path, req.URL.Path)
	}
}

func TestRemoveImageNotFound(t *testing.T) {
	client := Client{
		endpoint: "http://localhost:4243",
		client: &http.Client{
			Transport: &FakeRoundTripper{message: "no such image", status: http.StatusNotFound},
		},
	}
	err := client.RemoveImage("test:")
	if err != ErrNoSuchImage {
		t.Errorf("RemoveImage: wrong error. Want %#v. Got %#v.", ErrNoSuchImage, err)
	}
}

func TestInspectImage(t *testing.T) {
	body := `{
     "id":"b750fe79269d2ec9a3c593ef05b4332b1d1a02a62b4accb2c21d589ff2f5f2dc",
     "parent":"27cf784147099545",
     "created":"2013-03-23T22:24:18.818426-07:00",
     "container":"3d67245a8d72ecf13f33dffac9f79dcdf70f75acb84d308770391510e0c23ad0",
     "container_config":{"Memory":0}
}`
	var expected docker.Image
	json.Unmarshal([]byte(body), &expected)
	fakeRT := FakeRoundTripper{message: body, status: http.StatusOK}
	client := Client{endpoint: "http://localhost:4243", client: &http.Client{Transport: &fakeRT}}
	image, err := client.InspectImage(expected.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*image, expected) {
		t.Errorf("InspectImage(%q): Wrong image returned. Want %#v. Got %#v.", expected.ID, expected, *image)
	}
	req := fakeRT.requests[0]
	if req.Method != "GET" {
		t.Errorf("InspectImage(%q): Wrong HTTP method. Want GET. Got %s.", expected.ID, req.Method)
	}
	u, _ := url.Parse(client.getURL("/images/" + expected.ID + "/json"))
	if req.URL.Path != u.Path {
		t.Errorf("InspectImage(%q): Wrong request URL. Want %q. Got %q.", expected.ID, u.Path, req.URL.Path)
	}
}

func TestInspectImageNotFound(t *testing.T) {
	client := Client{
		endpoint: "http://localhost:4243",
		client: &http.Client{
			Transport: &FakeRoundTripper{message: "no such image", status: http.StatusNotFound},
		},
	}
	name := "test"
	image, err := client.InspectImage(name)
	if image != nil {
		t.Errorf("InspectImage(%q): expected <nil> image, got %#v.", name, image)
	}
	if err != ErrNoSuchImage {
		t.Errorf("InspectImage(%q): wrong error. Want %#v. Got %#v.", name, ErrNoSuchImage, err)
	}
}

func TestPushImage(t *testing.T) {
	fakeRT := FakeRoundTripper{message: "Pushing 1/100", status: http.StatusOK}
	client := Client{
		endpoint: "http://localhost:4243",
		client:   &http.Client{Transport: &fakeRT},
	}
	var buf bytes.Buffer
	err := client.PushImage(&PushImageOptions{Name: "test"}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	expected := "Pushing 1/100"
	if buf.String() != expected {
		t.Errorf("PushImage: Wrong output. Want %q. Got %q.", expected, buf.String())
	}
	req := fakeRT.requests[0]
	if req.Method != "POST" {
		t.Errorf("PushImage: Wrong HTTP method. Want POST. Got %s.", req.Method)
	}
	u, _ := url.Parse(client.getURL("/images/test/push"))
	if req.URL.Path != u.Path {
		t.Errorf("PushImage: Wrong request path. Want %q. Got %q.", u.Path, req.URL.Path)
	}
	if query := req.URL.Query().Encode(); query != "" {
		t.Errorf("PushImage: Wrong query stirng. Want no parameters, got %q.", query)
	}
}

func TestPushImageCustomRegistry(t *testing.T) {
	fakeRT := FakeRoundTripper{message: "Pushing 1/100", status: http.StatusOK}
	client := Client{
		endpoint: "http://localhost:4243",
		client:   &http.Client{Transport: &fakeRT},
	}
	var buf bytes.Buffer
	err := client.PushImage(&PushImageOptions{Name: "test", Registry: "docker.tsuru.io"}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	expectedQuery := "registry=docker.tsuru.io"
	if query := req.URL.Query().Encode(); query != expectedQuery {
		t.Errorf("PushImage: Wrong query string. Want %q. Got %q.", expectedQuery, query)
	}
}

func TestPushImageNoName(t *testing.T) {
	options := []*PushImageOptions{nil, {}}
	for _, opt := range options {
		client := Client{}
		err := client.PushImage(opt, nil)
		if err != ErrNoSuchImage {
			t.Errorf("PushImage: got wrong error. Want %#v. Got %#v.", ErrNoSuchImage, err)
		}
	}
}
