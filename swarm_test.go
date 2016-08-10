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

func TestSwarmInit(t *testing.T) {
	fakeRT := &FakeRoundTripper{message: `"body"`, status: http.StatusOK}
	client := newTestClient(fakeRT)
	response, err := client.SwarmInit(SwarmInitOptions{})
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

func TestSwarmJoin(t *testing.T) {
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	err := client.SwarmJoin(SwarmJoinOptions{})
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

func TestSwarmLeave(t *testing.T) {
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	var testData = []struct {
		force       bool
		expectedURI string
	}{
		{false, "/swarm/leave?"},
		{true, "/swarm/leave?force=1"},
	}
	for i, tt := range testData {
		err := client.SwarmLeave(SwarmLeaveOptions{Force: tt.force})
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

func TestSwarmUpdate(t *testing.T) {
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	opts := SwarmUpdateOptions{
		Version:            10,
		RotateManagerToken: true,
		RotateWorkerToken:  false,
	}
	err := client.SwarmUpdate(opts)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	expectedMethod := "POST"
	if req.Method != expectedMethod {
		t.Errorf("SwarmUpdate: Wrong HTTP method. Want %s. Got %s.", expectedMethod, req.Method)
	}
	expected, _ := url.Parse(client.getURL("/swarm/update?version=10&rotateManagerToken=true&rotateWorkerToken=false"))
	if req.URL.Path != expected.Path {
		t.Errorf("SwarmUpdate: Wrong request path. Want %q. Got %q.", expected.Path, req.URL.Path)
	}
	if !reflect.DeepEqual(req.URL.Query(), expected.Query()) {
		t.Errorf("SwarmUpdate: Wrong request query. Want %v. Got %v", expected.Query(), req.URL.Query())
	}
}
