// Copyright 2013 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testing

import (
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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

func TestListContainers(t *testing.T) {
	server := DockerServer{}
	addContainers(&server, 2)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("GET", "/v1.1/containers/json?all=1", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("ListContainers: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	expected := make([]docker.APIContainers, 2)
	for i, container := range server.containers {
		expected[i] = docker.APIContainers{
			ID:      container.ID,
			Image:   container.Image,
			Command: strings.Join(container.Config.Cmd, " "),
			Created: container.Created.Unix(),
			Status:  container.State.String(),
			Ports:   container.NetworkSettings.PortMappingHuman(),
		}
	}
	var got []docker.APIContainers
	err := json.NewDecoder(recorder.Body).Decode(&got)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("ListContainers. Want %#v. Got %#v.", expected, got)
	}
}

func TestCreateContainer(t *testing.T) {
	server := DockerServer{}
	server.imgIDs = map[string]string{"base": "a1234"}
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	body := `{"Hostname":"", "User":"", "Memory":0, "MemorySwap":0, "AttachStdin":false, "AttachStdout":true, "AttachStderr":true,
"PortSpecs":null, "Tty":false, "OpenStdin":false, "StdinOnce":false, "Env":null, "Cmd":["date"],
"Dns":null, "Image":"base", "Volumes":{}, "VolumesFrom":""}`
	request, _ := http.NewRequest("POST", "/v1.1/containers/create", strings.NewReader(body))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Errorf("CreateContainer: wrong status. Want %d. Got %d.", http.StatusCreated, recorder.Code)
	}
	var returned docker.Container
	err := json.NewDecoder(recorder.Body).Decode(&returned)
	if err != nil {
		t.Fatal(err)
	}
	stored := server.containers[0]
	if returned.ID != stored.ID {
		t.Errorf("CreateContainer: ID mismatch. Stored: %q. Returned: %q.", stored.ID, returned.ID)
	}
	if stored.State.Running {
		t.Errorf("CreateContainer should not set container to running state.")
	}
}

func TestCreateContainerInvalidBody(t *testing.T) {
	server := DockerServer{}
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("POST", "/v1.1/containers/create", strings.NewReader("whaaaaaat---"))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("CreateContainer: wrong status. Want %d. Got %d.", http.StatusBadRequest, recorder.Code)
	}
}

func TestCreateContainerImageNotFound(t *testing.T) {
	server := DockerServer{}
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	body := `{"Hostname":"", "User":"", "Memory":0, "MemorySwap":0, "AttachStdin":false, "AttachStdout":true, "AttachStderr":true,
"PortSpecs":null, "Tty":false, "OpenStdin":false, "StdinOnce":false, "Env":null, "Cmd":["date"],
"Dns":null, "Image":"base", "Volumes":{}, "VolumesFrom":""}`
	request, _ := http.NewRequest("POST", "/v1.1/containers/create", strings.NewReader(body))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("CreateContainer: wrong status. Want %d. Got %d.", http.StatusNotFound, recorder.Code)
	}
}

func TestCommitContainer(t *testing.T) {
	server := DockerServer{}
	addContainers(&server, 2)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("POST", "/v1.1/commit?container="+server.containers[0].ID, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("CommitContainer: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	expected := fmt.Sprintf(`{"ID":"%s"}`, server.images[0].ID)
	if got := recorder.Body.String(); got != expected {
		t.Errorf("CommitContainer: wrong response body. Want %q. Got %q.", expected, got)
	}
}

func TestCommitContainerComplete(t *testing.T) {
	server := DockerServer{}
	server.imgIDs = make(map[string]string)
	addContainers(&server, 2)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	queryString := "container=" + server.containers[0].ID + "&repo=tsuru/python&m=saving&author=developers"
	queryString += `&run={"Cmd": ["cat", "/world"],"PortSpecs":["22"]}`
	request, _ := http.NewRequest("POST", "/v1.1/commit?"+queryString, nil)
	server.ServeHTTP(recorder, request)
	image := server.images[0]
	if image.Parent != server.containers[0].Image {
		t.Errorf("CommitContainer: wrong parent image. Want %q. Got %q.", server.containers[0].Image, image.Parent)
	}
	if image.Container != server.containers[0].ID {
		t.Errorf("CommitContainer: wrong container. Want %q. Got %q.", server.containers[0].ID, image.Container)
	}
	message := "saving"
	if image.Comment != message {
		t.Errorf("CommitContainer: wrong comment (commit message). Want %q. Got %q.", message, image.Comment)
	}
	author := "developers"
	if image.Author != author {
		t.Errorf("CommitContainer: wrong author. Want %q. Got %q.", author, image.Author)
	}
	if id := server.imgIDs["tsuru/python"]; id != image.ID {
		t.Errorf("CommitContainer: wrong ID saved for repository. Want %q. Got %q.", image.ID, id)
	}
	portSpecs := []string{"22"}
	if !reflect.DeepEqual(image.Config.PortSpecs, portSpecs) {
		t.Errorf("CommitContainer: wrong port spec in config. Want %#v. Got %#v.", portSpecs, image.Config.PortSpecs)
	}
	cmd := []string{"cat", "/world"}
	if !reflect.DeepEqual(image.Config.Cmd, cmd) {
		t.Errorf("CommitContainer: wrong cmd in config. Want %#v. Got %#v.", cmd, image.Config.Cmd)
	}
}

func TestCommitContainerInvalidRun(t *testing.T) {
	server := DockerServer{}
	addContainers(&server, 1)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("POST", "/v1.1/commit?container="+server.containers[0].ID+"&run=abc---", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("CommitContainer. Wrong status. Want %d. Got %d.", http.StatusBadRequest, recorder.Code)
	}
}

func TestCommitContainerNotFound(t *testing.T) {
	server := DockerServer{}
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("POST", "/v1.1/commit?container=abc123", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("CommitContainer. Wrong status. Want %d. Got %d.", http.StatusNotFound, recorder.Code)
	}
}

func TestInspectContainer(t *testing.T) {
	server := DockerServer{}
	addContainers(&server, 2)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/v1.1/containers/%s/json", server.containers[0].ID)
	request, _ := http.NewRequest("GET", path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("InspectContainer: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	expected := server.containers[0]
	var got docker.Container
	err := json.NewDecoder(recorder.Body).Decode(&got)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, *expected) {
		t.Errorf("InspectContainer: wrong value. Want %#v. Got %#v.", *expected, got)
	}
}

func TestInspectContainerNotFound(t *testing.T) {
	server := DockerServer{}
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("GET", "/v1.1/containers/abc123/json", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("InspectContainer: wrong status code. Want %d. Got %d.", http.StatusNotFound, recorder.Code)
	}
}

func TestStartContainer(t *testing.T) {
	server := DockerServer{}
	addContainers(&server, 1)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/v1.1/containers/%s/start", server.containers[0].ID)
	request, _ := http.NewRequest("POST", path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("StartContainer: wrong status code. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	if !server.containers[0].State.Running {
		t.Error("StartContainer: did not set the container to running state")
	}
}

func TestStartContainerNotFound(t *testing.T) {
	server := DockerServer{}
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := "/v1.1/containers/abc123/start"
	request, _ := http.NewRequest("POST", path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("StartContainer: wrong status code. Want %d. Got %d.", http.StatusNotFound, recorder.Code)
	}
}

func TestAttachContainer(t *testing.T) {
	server := DockerServer{}
	addContainers(&server, 1)
	server.containers[0].State.Running = true
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/v1.1/containers/%s/attach?logs=1", server.containers[0].ID)
	request, _ := http.NewRequest("POST", path, nil)
	server.ServeHTTP(recorder, request)
	lines := []string{
		fmt.Sprintf("Container %q is running", server.containers[0].ID),
		"What happened?",
		"Something happened",
	}
	expected := strings.Join(lines, "\n") + "\n"
	if body := recorder.Body.String(); !reflect.DeepEqual(body, expected) {
		t.Errorf("AttachContainer: wrong body. Want %q. Got %q.", expected, body)
	}
}

func TestAttachContainerNotFound(t *testing.T) {
	server := DockerServer{}
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := "/v1.1/containers/abc123/attach?logs=1"
	request, _ := http.NewRequest("POST", path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("AttachContainer: wrong status. Want %d. Got %d.", http.StatusNotFound, recorder.Code)
	}
}

func TestPullImage(t *testing.T) {
	server := DockerServer{}
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("POST", "/v1.1/images/create?fromImage=base", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("PullImage: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	if len(server.images) != 1 {
		t.Errorf("PullImage: Want 1 image. Got %d.", len(server.images))
	}
}

func TestPullImageRepository(t *testing.T) {
	server := DockerServer{imgIDs: make(map[string]string)}
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("POST", "/v1.1/images/create?fromImage=base&repo=myrepo", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("PullImage: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	if _, ok := server.imgIDs["myrepo"]; !ok {
		t.Error("PullImage: Repository should not be empty.")
	}
}

func addContainers(server *DockerServer, n int) {
	server.cMut.Lock()
	defer server.cMut.Unlock()
	for i := 0; i < n; i++ {
		date := time.Now().Add(time.Duration((rand.Int() % (i + 1))) * time.Hour)
		container := docker.Container{
			ID:      fmt.Sprintf("%x", rand.Int()%10000),
			Created: date,
			Path:    "ls",
			Args:    []string{"-la", ".."},
			Config: &docker.Config{
				Hostname:     fmt.Sprintf("docker-%d", i),
				AttachStdout: true,
				AttachStderr: true,
				Env:          []string{"ME=you", fmt.Sprintf("NUMBER=%d", i)},
				Cmd:          []string{"ls", "-la", ".."},
				Dns:          nil,
				Image:        "base",
			},
			State: docker.State{
				Running:   false,
				Pid:       400 + i,
				ExitCode:  0,
				StartedAt: date,
			},
			Image: "b750fe79269d2ec9a3c593ef05b4332b1d1a02a62b4accb2c21d589ff2f5f2dc",
			NetworkSettings: &docker.NetworkSettings{
				IPAddress:   fmt.Sprintf("10.10.10.%d", i+2),
				IPPrefixLen: 24,
				Gateway:     "10.10.10.1",
				Bridge:      "docker0",
				PortMapping: map[string]string{"8888": fmt.Sprintf("%d", 49600+i)},
			},
			ResolvConfPath: "/etc/resolv.conf",
		}
		server.containers = append(server.containers, &container)
	}
}
