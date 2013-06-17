// Copyright 2013 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testing

import (
	"encoding/json"
	"github.com/dotcloud/docker"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
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
	server := DockerServer{}
	server.buildMuxer()
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

func TestInspectContainer(t *testing.T) {
	server := DockerServer{}
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("GET", "/v1.1/containers/abc123/json", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("InspectContainer: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	expected := docker.Container{
		ID:      "abc123",
		Created: time.Date(2013, time.June, 17, 10, 20, 0, 0, time.UTC),
		Path:    "date",
		Args:    []string{},
		Config: &docker.Config{
			Hostname:     "4fa6e0f0c678",
			User:         "",
			Memory:       67108864,
			MemorySwap:   0,
			AttachStdin:  false,
			AttachStdout: true,
			AttachStderr: true,
			PortSpecs:    nil,
			Tty:          false,
			OpenStdin:    false,
			StdinOnce:    false,
			Cmd:          []string{"date"},
			Dns:          nil,
			Image:        "base",
			Volumes:      map[string]struct{}{},
			VolumesFrom:  "",
		},
		State: docker.State{
			Running:   false,
			Pid:       0,
			ExitCode:  0,
			StartedAt: time.Date(2013, time.June, 17, 10, 21, 0, 0, time.UTC),
			Ghost:     false,
		},
		Image: "b750fe79269d2ec9a3c593ef05b4332b1d1a02a62b4accb2c21d589ff2f5f2dc",
		NetworkSettings: &docker.NetworkSettings{
			IPAddress:   "10.10.10.10",
			IPPrefixLen: 24,
			Gateway:     "10.10.10.1",
			Bridge:      "docker0",
			PortMapping: map[string]string{"8888": "32412"},
		},
	}
	var got docker.Container
	err := json.NewDecoder(recorder.Body).Decode(&got)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("InspectContainer: wrong value. Want %#v. Got %#v.", expected, got)
	}
}
