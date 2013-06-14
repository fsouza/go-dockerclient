// Copyright 2013 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testing

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewServer(t *testing.T) {
	server, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.listener.Close()
	conn, err := net.Dial("tcp", server.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
}

func TestServerStop(t *testing.T) {
	server, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	server.Stop()
	_, err = net.Dial("tcp", server.listener.Addr().String())
	if err == nil {
		t.Error("Unexpected <nil> error when dialing to stopped server")
	}
}

func TestServerStopNoListener(t *testing.T) {
	server := DockerServer{}
	server.Stop()
}

func TestServerURL(t *testing.T) {
	server, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()
	url := server.URL()
	if expected := "http://" + server.listener.Addr().String() + "/"; url != expected {
		t.Errorf("DockerServer.URL(): Want %q. Got %q.", expected, url)
	}
}

func TestServerURLNoListener(t *testing.T) {
	server := DockerServer{}
	url := server.URL()
	if url != "" {
		t.Errorf("DockerServer.URL(): Expected empty URL on handler mode, got %q.", url)
	}
}

func TestCommitContainer(t *testing.T) {
	server, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("POST", "/v1.1/commit?container=abcdef", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("CommitContainer: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	expected := `{"ID":"abcdef"}`
	if got := recorder.Body.String(); got != expected {
		t.Errorf("CommitContainer: wrong response body. Want %q. Got %q.", expected, got)
	}
}
