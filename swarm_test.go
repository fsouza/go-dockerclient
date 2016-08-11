// Copyright 2016 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

func TestInitSwarm(t *testing.T) {
	fakeRT := &FakeRoundTripper{message: `"body"`, status: http.StatusOK}
	client := newTestClient(fakeRT)
	response, err := client.InitSwarm(InitSwarmOptions{})
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	expectedMethod := "POST"
	if req.Method != expectedMethod {
		t.Errorf("SwarmInit: Wrong HTTP method. Want %s. Got %s.", expectedMethod, req.Method)
	}
	u, _ := url.Parse(client.getURL("/swarm/init"))
	if req.URL.Path != u.Path {
		t.Errorf("SwarmInit: Wrong request path. Want %q. Got %q.", u.Path, req.URL.Path)
	}
	expected := "body"
	if response != expected {
		t.Errorf("SwarmInit: Wrong response. Want %q. Got %q.", expected, response)
	}
}

func TestJoinSwarm(t *testing.T) {
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	err := client.JoinSwarm(JoinSwarmOptions{})
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	expectedMethod := "POST"
	if req.Method != expectedMethod {
		t.Errorf("SwarmJoin: Wrong HTTP method. Want %s. Got %s.", expectedMethod, req.Method)
	}
	u, _ := url.Parse(client.getURL("/swarm/join"))
	if req.URL.Path != u.Path {
		t.Errorf("SwarmJoin: Wrong request path. Want %q. Got %q.", u.Path, req.URL.Path)
	}
}

func TestLeaveSwarm(t *testing.T) {
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	var testData = []struct {
		force       bool
		expectedURI string
	}{
		{false, "/swarm/leave?force=false"},
		{true, "/swarm/leave?force=true"},
	}
	for i, tt := range testData {
		err := client.LeaveSwarm(LeaveSwarmOptions{Force: tt.force})
		if err != nil {
			t.Fatal(err)
		}
		expectedMethod := "POST"
		req := fakeRT.requests[i]
		if req.Method != expectedMethod {
			t.Errorf("SwarmLeave: Wrong HTTP method. Want %s. Got %s.", expectedMethod, req.Method)
		}
		expected, _ := url.Parse(client.getURL(tt.expectedURI))
		if req.URL.String() != expected.String() {
			t.Errorf("SwarmLeave: Wrong request string. Want %q. Got %q.", expected.String(), req.URL.String())
		}
	}
}

func TestUpdateSwarm(t *testing.T) {
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	opts := UpdateSwarmOptions{
		Version:            10,
		RotateManagerToken: true,
		RotateWorkerToken:  false,
	}
	err := client.UpdateSwarm(opts)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	expectedMethod := "POST"
	if req.Method != expectedMethod {
		t.Errorf("SwarmUpdate: Wrong HTTP method. Want %s. Got %s.", expectedMethod, req.Method)
	}
	expectedPath := "/swarm/update"
	if req.URL.Path != expectedPath {
		t.Errorf("SwarmUpdate: Wrong request path. Want %q. Got %q.", expectedPath, req.URL.Path)
	}
	expected := map[string][]string{
		"version":            {"10"},
		"rotateManagerToken": {"true"},
		"rotateWorkerToken":  {"false"},
	}
	got := map[string][]string(req.URL.Query())
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("SwarmUpdate: Wrong request query. Want %v. Got %v", expected, got)
	}
}
