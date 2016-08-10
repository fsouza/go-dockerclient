// Copyright 2016 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/docker/engine-api/types/swarm"
)

func TestSwarmInit(t *testing.T) {
	fakeRT := &FakeRoundTripper{message: `"body"`, status: http.StatusOK}
	client := newTestClient(fakeRT)
	response, err := client.SwarmInit(swarm.InitRequest{})
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
