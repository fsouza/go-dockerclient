// Copyright 2013 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testing

import (
	"archive/tar"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/swarm"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/fsouza/go-dockerclient/internal/testutils"
)

func TestNewServer(t *testing.T) {
	t.Parallel()
	server, err := NewServer("127.0.0.1:0", nil, nil)
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

func TestNewTLSServer(t *testing.T) {
	t.Parallel()
	caCert, serverCert := testutils.GenCertificate(t)

	tlsConfig := TLSConfig{
		CertPath:    serverCert.CertPath,
		CertKeyPath: serverCert.KeyPath,
		RootCAPath:  caCert.CertPath,
	}
	server, err := NewTLSServer("127.0.0.1:0", nil, nil, tlsConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer server.listener.Close()
	conn, err := net.Dial("tcp", server.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	client, err := docker.NewTLSClient(server.URL(), "./data/cert.pem", "./data/key.pem", "./data/ca.pem")
	if err != nil {
		t.Fatal(err)
	}
	err = client.Ping()
	if err != nil {
		t.Fatal(err)
	}
}

func TestServerStop(t *testing.T) {
	t.Parallel()
	const retries = 3
	server, err := NewServer("127.0.0.1:0", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	server.Stop()
	_, err = net.Dial("tcp", server.listener.Addr().String())
	for i := 0; i < retries && err == nil; i++ {
		time.Sleep(100 * time.Millisecond)
		_, err = net.Dial("tcp", server.listener.Addr().String())
	}
	if err == nil {
		t.Error("Unexpected <nil> error when dialing to stopped server")
	}
}

func TestServerStopNoListener(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.Stop()
}

func TestServerURL(t *testing.T) {
	t.Parallel()
	server, err := NewServer("127.0.0.1:0", nil, nil)
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
	t.Parallel()
	server := baseDockerServer()
	url := server.URL()
	if url != "" {
		t.Errorf("DockerServer.URL(): Expected empty URL on handler mode, got %q.", url)
	}
}

func TestHandleWithHook(t *testing.T) {
	t.Parallel()
	var called bool
	server, _ := NewServer("127.0.0.1:0", nil, func(*http.Request) { called = true })
	defer server.Stop()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/containers/json?all=1", nil)
	server.ServeHTTP(recorder, request)
	if !called {
		t.Error("ServeHTTP did not call the hook function.")
	}
}

func TestSetHook(t *testing.T) {
	t.Parallel()
	var called bool
	server, _ := NewServer("127.0.0.1:0", nil, nil)
	defer server.Stop()
	server.SetHook(func(*http.Request) { called = true })
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/containers/json?all=1", nil)
	server.ServeHTTP(recorder, request)
	if !called {
		t.Error("ServeHTTP did not call the hook function.")
	}
}

func TestCustomHandler(t *testing.T) {
	t.Parallel()
	var called bool
	server, _ := NewServer("127.0.0.1:0", nil, nil)
	defer server.Stop()
	addContainers(server, 2)
	server.CustomHandler("/containers/json", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		fmt.Fprint(w, "Hello world")
	}))
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/containers/json?all=1", nil)
	server.ServeHTTP(recorder, request)
	if !called {
		t.Error("Did not call the custom handler")
	}
	if got := recorder.Body.String(); got != "Hello world" {
		t.Errorf("Wrong output for custom handler: want %q. Got %q.", "Hello world", got)
	}
}

func TestCustomHandlerRegexp(t *testing.T) {
	t.Parallel()
	var called bool
	server, _ := NewServer("127.0.0.1:0", nil, nil)
	defer server.Stop()
	addContainers(server, 2)
	server.CustomHandler("/containers/.*/json", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		fmt.Fprint(w, "Hello world")
	}))
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/containers/.*/json?all=1", nil)
	server.ServeHTTP(recorder, request)
	if !called {
		t.Error("Did not call the custom handler")
	}
	if got := recorder.Body.String(); got != "Hello world" {
		t.Errorf("Wrong output for custom handler: want %q. Got %q.", "Hello world", got)
	}
}

func TestListContainers(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	containers := addContainers(&server, 2)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/containers/json?all=1", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("ListContainers: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	expected := make([]docker.APIContainers, 2)
	for i, container := range containers {
		expected[i] = docker.APIContainers{
			ID:      container.ID,
			Image:   container.Image,
			Command: strings.Join(container.Config.Cmd, " "),
			Created: container.Created.Unix(),
			Status:  container.State.String(),
			Ports:   container.NetworkSettings.PortMappingAPI(),
			Names:   []string{"/" + container.Name},
			State:   container.State.StateString(),
			Labels:  map[string]string{"key": fmt.Sprintf("val-%d", i)},
		}
	}
	sortFn := func(left, right docker.APIContainers) int {
		return strings.Compare(left.ID, right.ID)
	}
	slices.SortFunc(expected, sortFn)
	var got []docker.APIContainers
	err := json.NewDecoder(recorder.Body).Decode(&got)
	if err != nil {
		t.Fatal(err)
	}
	slices.SortFunc(got, sortFn)
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("ListContainers.\nWant %#v.\nGot  %#v.", expected, got)
	}
}

func TestListRunningContainers(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 2)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/containers/json?all=0", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("ListRunningContainers: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	var got []docker.APIContainers
	err := json.NewDecoder(recorder.Body).Decode(&got)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("ListRunningContainers: Want 0. Got %d.", len(got))
	}
}

func TestListContainersFilterLabels(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 3)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	filters := url.QueryEscape(`{"label": ["key=val-1"]}`)
	request, _ := http.NewRequest(http.MethodGet, "/containers/json?all=1&filters="+filters, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("TestListContainersFilterLabels: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	var got []docker.APIContainers
	err := json.NewDecoder(recorder.Body).Decode(&got)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("TestListContainersFilterLabels: Want 1. Got %d.", len(got))
	}
	filters = url.QueryEscape(`{"label": ["key="]}`)
	request, _ = http.NewRequest(http.MethodGet, "/containers/json?all=1&filters="+filters, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("TestListContainersFilterLabels: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	err = json.NewDecoder(recorder.Body).Decode(&got)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("TestListContainersFilterLabels: Want 0. Got %d.", len(got))
	}
	filters = url.QueryEscape(`{"label": ["key"]}`)
	request, _ = http.NewRequest(http.MethodGet, "/containers/json?all=1&filters="+filters, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("TestListContainersFilterLabels: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	err = json.NewDecoder(recorder.Body).Decode(&got)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Errorf("TestListContainersFilterLabels: Want 3. Got %d.", len(got))
	}
}

func TestCreateContainer(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.imgIDs = map[string]string{"base": "a1234"}
	server.uploadedFiles = map[string]string{"a1234": "/abcd"}
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	body := `{"Hostname":"", "User":"ubuntu", "Memory":0, "MemorySwap":0, "AttachStdin":false, "AttachStdout":true, "AttachStderr":true,
"PortSpecs":null, "Tty":false, "OpenStdin":false, "StdinOnce":false, "Env":null, "Cmd":["date"], "Image":"base", "Volumes":{}, "VolumesFrom":"","HostConfig":{"Binds":["/var/run/docker.sock:/var/run/docker.sock:rw"]}}`
	request, _ := http.NewRequest(http.MethodPost, "/containers/create", strings.NewReader(body))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Errorf("CreateContainer: wrong status. Want %d. Got %d.", http.StatusCreated, recorder.Code)
	}
	var returned docker.Container
	err := json.NewDecoder(recorder.Body).Decode(&returned)
	if err != nil {
		t.Fatal(err)
	}
	stored := getContainer(&server)
	if returned.ID != stored.ID {
		t.Errorf("CreateContainer: ID mismatch. Stored: %q. Returned: %q.", stored.ID, returned.ID)
	}
	if stored.State.Running {
		t.Errorf("CreateContainer should not set container to running state.")
	}
	if !stored.State.StartedAt.IsZero() {
		t.Errorf("CreateContainer should not set startedAt in container state.")
	}
	if stored.Config.User != "ubuntu" {
		t.Errorf("CreateContainer: wrong config. Expected: %q. Returned: %q.", "ubuntu", stored.Config.User)
	}
	if stored.Config.Hostname != returned.ID[:12] {
		t.Errorf("CreateContainer: wrong hostname. Expected: %q. Returned: %q.", returned.ID[:12], stored.Config.Hostname)
	}
	expectedBind := []string{"/var/run/docker.sock:/var/run/docker.sock:rw"}
	if !reflect.DeepEqual(stored.HostConfig.Binds, expectedBind) {
		t.Errorf("CreateContainer: wrong host config. Expected: %v. Returned %v.", expectedBind, stored.HostConfig.Binds)
	}
	if val, ok := server.uploadedFiles[stored.ID]; !ok {
		t.Error("CreateContainer: uploadedFiles should exist.")
	} else if val != "/abcd" {
		t.Errorf("CreateContainer: wrong uploadedFile. Want '/abcd', got %s.", val)
	}
}

func getContainer(server *DockerServer) *docker.Container {
	var cont *docker.Container
	for _, cont = range server.containers {
	}
	return cont
}

func TestCreateContainerWithNotifyChannel(t *testing.T) {
	t.Parallel()
	ch := make(chan *docker.Container, 1)
	server := baseDockerServer()
	server.imgIDs = map[string]string{"base": "a1234"}
	server.cChan = ch
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	body := `{"Hostname":"", "User":"", "Memory":0, "MemorySwap":0, "AttachStdin":false, "AttachStdout":true, "AttachStderr":true,
"PortSpecs":null, "Tty":false, "OpenStdin":false, "StdinOnce":false, "Env":null, "Cmd":["date"], "Image":"base", "Volumes":{}, "VolumesFrom":""}`
	request, _ := http.NewRequest(http.MethodPost, "/containers/create", strings.NewReader(body))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Errorf("CreateContainer: wrong status. Want %d. Got %d.", http.StatusCreated, recorder.Code)
	}
	if notified := <-ch; notified != getContainer(&server) {
		t.Errorf("CreateContainer: did not notify the proper container. Want %q. Got %q.", getContainer(&server).ID, notified.ID)
	}
}

func TestCreateContainerInvalidBody(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPost, "/containers/create", strings.NewReader("whaaaaaat---"))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("CreateContainer: wrong status. Want %d. Got %d.", http.StatusBadRequest, recorder.Code)
	}
}

func TestCreateContainerDuplicateName(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	server.imgIDs = map[string]string{"base": "a1234"}
	containers := addContainers(&server, 1)
	containers[0].Name = "mycontainer"
	server.contNameToID[containers[0].Name] = containers[0].ID
	recorder := httptest.NewRecorder()
	body := `{"Hostname":"", "User":"ubuntu", "Memory":0, "MemorySwap":0, "AttachStdin":false, "AttachStdout":true, "AttachStderr":true,
"PortSpecs":null, "Tty":false, "OpenStdin":false, "StdinOnce":false, "Env":null, "Cmd":["date"], "Image":"base", "Volumes":{}, "VolumesFrom":"","HostConfig":{"Binds":["/var/run/docker.sock:/var/run/docker.sock:rw"]}}`
	request, _ := http.NewRequest(http.MethodPost, "/containers/create?name=mycontainer", strings.NewReader(body))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Errorf("CreateContainer: wrong status. Want %d. Got %d.", http.StatusConflict, recorder.Code)
	}
}

func TestCreateMultipleContainersEmptyName(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	server.imgIDs = map[string]string{"base": "a1234"}
	addContainers(&server, 1)
	getContainer(&server).Name = ""
	recorder := httptest.NewRecorder()
	body := `{"Hostname":"", "User":"ubuntu", "Memory":0, "MemorySwap":0, "AttachStdin":false, "AttachStdout":true, "AttachStderr":true,
"PortSpecs":null, "Tty":false, "OpenStdin":false, "StdinOnce":false, "Env":null, "Cmd":["date"], "Image":"base", "Volumes":{}, "VolumesFrom":"","HostConfig":{"Binds":["/var/run/docker.sock:/var/run/docker.sock:rw"]}}`
	request, _ := http.NewRequest(http.MethodPost, "/containers/create", strings.NewReader(body))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Errorf("CreateContainer: wrong status. Want %d. Got %d.", http.StatusCreated, recorder.Code)
	}
	var returned docker.Container
	err := json.NewDecoder(recorder.Body).Decode(&returned)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := server.findContainer(returned.ID)
	if err != nil {
		t.Fatal(err)
	}
	if returned.ID != stored.ID {
		t.Errorf("CreateContainer: ID mismatch. Stored: %q. Returned: %q.", stored.ID, returned.ID)
	}
	if stored.State.Running {
		t.Errorf("CreateContainer should not set container to running state.")
	}
	if stored.Config.User != "ubuntu" {
		t.Errorf("CreateContainer: wrong config. Expected: %q. Returned: %q.", "ubuntu", stored.Config.User)
	}
	expectedBind := []string{"/var/run/docker.sock:/var/run/docker.sock:rw"}
	if !reflect.DeepEqual(stored.HostConfig.Binds, expectedBind) {
		t.Errorf("CreateContainer: wrong host config. Expected: %v. Returned %v.", expectedBind, stored.HostConfig.Binds)
	}
}

func TestCreateContainerInvalidName(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	body := `{"Hostname":"", "User":"", "Memory":0, "MemorySwap":0, "AttachStdin":false, "AttachStdout":true, "AttachStderr":true,
"PortSpecs":null, "Tty":false, "OpenStdin":false, "StdinOnce":false, "Env":null, "Cmd":["date"],
"Image":"base", "Volumes":{}, "VolumesFrom":""}`
	request, _ := http.NewRequest(http.MethodPost, "/containers/create?name=myapp/container1", strings.NewReader(body))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("CreateContainer: wrong status. Want %d. Got %d.", http.StatusInternalServerError, recorder.Code)
	}
	expectedBody := "Invalid container name\n"
	if got := recorder.Body.String(); got != expectedBody {
		t.Errorf("CreateContainer: wrong body. Want %q. Got %q.", expectedBody, got)
	}
}

func TestCreateContainerImageNotFound(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	body := `{"Hostname":"", "User":"", "Memory":0, "MemorySwap":0, "AttachStdin":false, "AttachStdout":true, "AttachStderr":true,
"PortSpecs":null, "Tty":false, "OpenStdin":false, "StdinOnce":false, "Env":null, "Cmd":["date"],
"Image":"base", "Volumes":{}, "VolumesFrom":""}`
	request, _ := http.NewRequest(http.MethodPost, "/containers/create", strings.NewReader(body))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("CreateContainer: wrong status. Want %d. Got %d.", http.StatusNotFound, recorder.Code)
	}
}

func TestRenameContainer(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	containers := addContainers(&server, 2)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	newName := containers[0].Name + "abc"
	path := fmt.Sprintf("/containers/%s/rename?name=%s", containers[0].ID, newName)
	request, _ := http.NewRequest(http.MethodPost, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Errorf("RenameContainer: wrong status. Want %d. Got %d.", http.StatusNoContent, recorder.Code)
	}
	container := containers[0]
	if container.Name != newName {
		t.Errorf("RenameContainer: did not rename the container. Want %q. Got %q.", newName, container.Name)
	}
}

func TestRenameContainerNotFound(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPost, "/containers/blabla/rename?name=something", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("RenameContainer: wrong status. Want %d. Got %d.", http.StatusNotFound, recorder.Code)
	}
}

func TestCommitContainer(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	containers := addContainers(&server, 2)
	server.uploadedFiles = map[string]string{containers[0].ID: "/abcd"}
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPost, "/commit?container="+containers[0].ID, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("CommitContainer: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	if len(server.images) != 1 {
		t.Errorf("CommitContainer: wrong images len in server. Want 1. Got %q.", len(server.images))
	}
	imgID := fmt.Sprintf("img-%s", containers[0].ID)
	expected := fmt.Sprintf(`{"ID":"%s"}`, imgID)
	if got := recorder.Body.String(); got != expected {
		t.Errorf("CommitContainer: wrong response body. Want %q. Got %q.", expected, got)
	}
	if server.images[imgID].Config == nil {
		t.Error("CommitContainer: image Config should not be nil.")
	}
	if val, ok := server.uploadedFiles[server.images[imgID].ID]; !ok {
		t.Error("CommitContainer: uploadedFiles should exist.")
	} else if val != "/abcd" {
		t.Errorf("CommitContainer: wrong uploadedFile. Want '/abcd', got %s.", val)
	}
}

func TestCommitContainerComplete(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	containers := addContainers(&server, 2)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	qs := make(url.Values)
	qs.Add("container", containers[0].ID)
	qs.Add("repo", "tsuru/python")
	qs.Add("m", "saving")
	qs.Add("author", "developers")
	qs.Add("run", `{"Cmd": ["cat", "/world"],"PortSpecs":["22"]}`)
	request, _ := http.NewRequest(http.MethodPost, "/commit?"+qs.Encode(), nil)
	server.ServeHTTP(recorder, request)
	imgID := fmt.Sprintf("img-%s", containers[0].ID)
	image := server.images[imgID]
	if image.Parent != containers[0].Image {
		t.Errorf("CommitContainer: wrong parent image. Want %q. Got %q.", containers[0].Image, image.Parent)
	}
	if image.Container != containers[0].ID {
		t.Errorf("CommitContainer: wrong container. Want %q. Got %q.", containers[0].ID, image.Container)
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

func TestCommitContainerWithTag(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	containers := addContainers(&server, 2)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	queryString := "container=" + containers[0].ID + "&repo=tsuru/python&tag=v1"
	request, _ := http.NewRequest(http.MethodPost, "/commit?"+queryString, nil)
	server.ServeHTTP(recorder, request)
	imgID := fmt.Sprintf("img-%s", containers[0].ID)
	image := server.images[imgID]
	if image.Parent != containers[0].Image {
		t.Errorf("CommitContainer: wrong parent image. Want %q. Got %q.", containers[0].Image, image.Parent)
	}
	if image.Container != containers[0].ID {
		t.Errorf("CommitContainer: wrong container. Want %q. Got %q.", containers[0].ID, image.Container)
	}
	if id := server.imgIDs["tsuru/python:v1"]; id != image.ID {
		t.Errorf("CommitContainer: wrong ID saved for repository. Want %q. Got %q.", image.ID, id)
	}
}

func TestCommitContainerInvalidRun(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPost, "/commit?container="+getContainer(&server).ID+"&run=abc---", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("CommitContainer. Wrong status. Want %d. Got %d.", http.StatusBadRequest, recorder.Code)
	}
}

func TestCommitContainerNotFound(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPost, "/commit?container=abc123", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("CommitContainer. Wrong status. Want %d. Got %d.", http.StatusNotFound, recorder.Code)
	}
}

func TestInspectContainer(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	containers := addContainers(&server, 2)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s/json", containers[0].ID)
	request, _ := http.NewRequest(http.MethodGet, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("InspectContainer: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	expected := containers[0]
	var got docker.Container
	err := json.NewDecoder(recorder.Body).Decode(&got)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Config, expected.Config) {
		t.Errorf("InspectContainer: wrong value. Want %#v. Got %#v.", *expected, got)
	}
	if !reflect.DeepEqual(got.NetworkSettings, expected.NetworkSettings) {
		t.Errorf("InspectContainer: wrong value. Want %#v. Got %#v.", *expected, got)
	}
	got.State.StartedAt = expected.State.StartedAt
	got.State.FinishedAt = expected.State.FinishedAt
	got.Config = expected.Config
	got.Created = expected.Created
	got.NetworkSettings = expected.NetworkSettings
	if !reflect.DeepEqual(got, *expected) {
		t.Errorf("InspectContainer: wrong value. Want %#v. Got %#v.", *expected, got)
	}
}

func TestInspectContainerNotFound(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/containers/abc123/json", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("InspectContainer: wrong status code. Want %d. Got %d.", http.StatusNotFound, recorder.Code)
	}
}

func TestTopContainer(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	getContainer(&server).State.Running = true
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s/top", getContainer(&server).ID)
	request, _ := http.NewRequest(http.MethodGet, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("TopContainer: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	var got docker.TopResult
	err := json.NewDecoder(recorder.Body).Decode(&got)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Titles, []string{"UID", "PID", "PPID", "C", "STIME", "TTY", "TIME", "CMD"}) {
		t.Fatalf("TopContainer: Unexpected titles, got: %#v", got.Titles)
	}
	if len(got.Processes) != 1 {
		t.Fatalf("TopContainer: Unexpected process len, got: %d", len(got.Processes))
	}
	if got.Processes[0][len(got.Processes[0])-1] != "ls -la .." {
		t.Fatalf("TopContainer: Unexpected command name, got: %s", got.Processes[0][len(got.Processes[0])-1])
	}
}

func TestTopContainerNotFound(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/containers/xyz/top", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("TopContainer: wrong status. Want %d. Got %d.", http.StatusNotFound, recorder.Code)
	}
}

func TestTopContainerStopped(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s/top", getContainer(&server).ID)
	request, _ := http.NewRequest(http.MethodGet, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("TopContainer: wrong status. Want %d. Got %d.", http.StatusInternalServerError, recorder.Code)
	}
}

func TestStartContainer(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	server.buildMuxer()
	memory := int64(536870912)
	hostConfig := docker.HostConfig{Memory: memory}
	configBytes, err := json.Marshal(hostConfig)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s/start", getContainer(&server).ID)
	request, _ := http.NewRequest(http.MethodPost, path, bytes.NewBuffer(configBytes))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("StartContainer: wrong status code. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	if !getContainer(&server).State.Running {
		t.Error("StartContainer: did not set the container to running state")
	}
	if getContainer(&server).State.StartedAt.IsZero() {
		t.Error("StartContainer: did not set the startedAt container state")
	}
	if gotMemory := getContainer(&server).HostConfig.Memory; gotMemory != memory {
		t.Errorf("StartContainer: wrong HostConfig. Wants %d of memory. Got %d", memory, gotMemory)
	}
}

func TestStartContainerNoHostConfig(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	server.buildMuxer()
	memory := int64(536870912)
	hostConfig := docker.HostConfig{Memory: memory}
	getContainer(&server).HostConfig = &hostConfig
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s/start", getContainer(&server).ID)
	request, _ := http.NewRequest(http.MethodPost, path, strings.NewReader(""))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("StartContainer: wrong status code. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	if !getContainer(&server).State.Running {
		t.Error("StartContainer: did not set the container to running state")
	}
	if gotMemory := getContainer(&server).HostConfig.Memory; gotMemory != memory {
		t.Errorf("StartContainer: wrong HostConfig. Wants %d of memory. Got %d", memory, gotMemory)
	}
}

func TestStartContainerChangeNetwork(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	server.buildMuxer()
	hostConfig := docker.HostConfig{
		PortBindings: map[docker.Port][]docker.PortBinding{
			"8888/tcp": {{HostIP: "", HostPort: "12345"}},
		},
	}
	configBytes, err := json.Marshal(hostConfig)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s/start", getContainer(&server).ID)
	request, _ := http.NewRequest(http.MethodPost, path, bytes.NewBuffer(configBytes))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("StartContainer: wrong status code. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	if !getContainer(&server).State.Running {
		t.Error("StartContainer: did not set the container to running state")
	}
	portMapping := getContainer(&server).NetworkSettings.Ports["8888/tcp"]
	expected := []docker.PortBinding{{HostIP: "0.0.0.0", HostPort: "12345"}}
	if !reflect.DeepEqual(portMapping, expected) {
		t.Errorf("StartContainer: network not updated. Wants %#v ports. Got %#v", expected, portMapping)
	}
}

func TestStartContainerWithNotifyChannel(t *testing.T) {
	t.Parallel()
	ch := make(chan *docker.Container, 1)
	server := baseDockerServer()
	server.cChan = ch
	containers := addContainers(&server, 2)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s/start", containers[1].ID)
	request, _ := http.NewRequest(http.MethodPost, path, bytes.NewBuffer([]byte("{}")))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("StartContainer: wrong status code. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	if notified := <-ch; notified != containers[1] {
		t.Errorf("StartContainer: did not notify the proper container. Want %q. Got %q.", containers[1].ID, notified.ID)
	}
}

func TestStartContainerNotFound(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := "/containers/abc123/start"
	request, _ := http.NewRequest(http.MethodPost, path, bytes.NewBuffer([]byte("null")))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("StartContainer: wrong status code. Want %d. Got %d.", http.StatusNotFound, recorder.Code)
	}
}

func TestStartContainerAlreadyRunning(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	getContainer(&server).State.Running = true
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s/start", getContainer(&server).ID)
	request, _ := http.NewRequest(http.MethodPost, path, bytes.NewBuffer([]byte("null")))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotModified {
		t.Errorf("StartContainer: wrong status code. Want %d. Got %d.", http.StatusNotModified, recorder.Code)
	}
}

func TestStopContainer(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	getContainer(&server).State.Running = true
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s/stop", getContainer(&server).ID)
	request, _ := http.NewRequest(http.MethodPost, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Errorf("StopContainer: wrong status code. Want %d. Got %d.", http.StatusNoContent, recorder.Code)
	}
	if getContainer(&server).State.Running {
		t.Error("StopContainer: did not stop the container")
	}
}

func TestKillContainer(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	getContainer(&server).State.Running = true
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s/kill", getContainer(&server).ID)
	request, _ := http.NewRequest(http.MethodPost, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Errorf("KillContainer: wrong status code. Want %d. Got %d.", http.StatusNoContent, recorder.Code)
	}
	if getContainer(&server).State.Running {
		t.Error("KillContainer: did not stop the container")
	}
}

func TestStopContainerWithNotifyChannel(t *testing.T) {
	t.Parallel()
	ch := make(chan *docker.Container, 1)
	server := baseDockerServer()
	server.cChan = ch
	containers := addContainers(&server, 2)
	containers[1].State.Running = true
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s/stop", containers[1].ID)
	request, _ := http.NewRequest(http.MethodPost, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Errorf("StopContainer: wrong status code. Want %d. Got %d.", http.StatusNoContent, recorder.Code)
	}
	if notified := <-ch; notified != containers[1] {
		t.Errorf("StopContainer: did not notify the proper container. Want %q. Got %q.", containers[1].ID, notified.ID)
	}
}

func TestStopContainerNotFound(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := "/containers/abc123/stop"
	request, _ := http.NewRequest(http.MethodPost, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("StopContainer: wrong status code. Want %d. Got %d.", http.StatusNotFound, recorder.Code)
	}
}

func TestStopContainerNotRunning(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s/stop", getContainer(&server).ID)
	request, _ := http.NewRequest(http.MethodPost, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("StopContainer: wrong status code. Want %d. Got %d.", http.StatusBadRequest, recorder.Code)
	}
}

func TestPauseContainer(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s/pause", getContainer(&server).ID)
	request, _ := http.NewRequest(http.MethodPost, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Errorf("PauseContainer: wrong status code. Want %d. Got %d.", http.StatusNoContent, recorder.Code)
	}
	if !getContainer(&server).State.Paused {
		t.Error("PauseContainer: did not pause the container")
	}
}

func TestPauseContainerAlreadyPaused(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	getContainer(&server).State.Paused = true
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s/pause", getContainer(&server).ID)
	request, _ := http.NewRequest(http.MethodPost, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("PauseContainer: wrong status code. Want %d. Got %d.", http.StatusBadRequest, recorder.Code)
	}
}

func TestPauseContainerNotFound(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := "/containers/abc123/pause"
	request, _ := http.NewRequest(http.MethodPost, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("PauseContainer: wrong status code. Want %d. Got %d.", http.StatusNotFound, recorder.Code)
	}
}

func TestUnpauseContainer(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	getContainer(&server).State.Paused = true
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s/unpause", getContainer(&server).ID)
	request, _ := http.NewRequest(http.MethodPost, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Errorf("UnpauseContainer: wrong status code. Want %d. Got %d.", http.StatusNoContent, recorder.Code)
	}
	if getContainer(&server).State.Paused {
		t.Error("UnpauseContainer: did not unpause the container")
	}
}

func TestUnpauseContainerNotPaused(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s/unpause", getContainer(&server).ID)
	request, _ := http.NewRequest(http.MethodPost, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("UnpauseContainer: wrong status code. Want %d. Got %d.", http.StatusBadRequest, recorder.Code)
	}
}

func TestUnpauseContainerNotFound(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := "/containers/abc123/unpause"
	request, _ := http.NewRequest(http.MethodPost, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("UnpauseContainer: wrong status code. Want %d. Got %d.", http.StatusNotFound, recorder.Code)
	}
}

func TestWaitContainer(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	getContainer(&server).State.Running = true
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s/wait", getContainer(&server).ID)
	request, _ := http.NewRequest(http.MethodPost, path, nil)
	go func() {
		server.cMut.Lock()
		getContainer(&server).State.Running = false
		server.cMut.Unlock()
	}()
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("WaitContainer: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	expected := `{"StatusCode":0}` + "\n"
	if body := recorder.Body.String(); body != expected {
		t.Errorf("WaitContainer: wrong body. Want %q. Got %q.", expected, body)
	}
}

func TestWaitContainerStatus(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	server.buildMuxer()
	getContainer(&server).State.ExitCode = 63
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s/wait", getContainer(&server).ID)
	request, _ := http.NewRequest(http.MethodPost, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("WaitContainer: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	expected := `{"StatusCode":63}` + "\n"
	if body := recorder.Body.String(); body != expected {
		t.Errorf("WaitContainer: wrong body. Want %q. Got %q.", expected, body)
	}
}

func TestWaitContainerNotFound(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := "/containers/abc123/wait"
	request, _ := http.NewRequest(http.MethodPost, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("WaitContainer: wrong status code. Want %d. Got %d.", http.StatusNotFound, recorder.Code)
	}
}

type HijackableResponseRecorder struct {
	httptest.ResponseRecorder
	readCh chan []byte
}

func (r *HijackableResponseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	myConn, otherConn := net.Pipe()
	r.readCh = make(chan []byte)
	go func() {
		data, _ := io.ReadAll(myConn)
		r.readCh <- data
	}()
	return otherConn, nil, nil
}

func (r *HijackableResponseRecorder) HijackBuffer() string {
	return string(<-r.readCh)
}

func TestLogContainer(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	getContainer(&server).State.Running = true
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s/logs", getContainer(&server).ID)
	request, _ := http.NewRequest(http.MethodGet, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("LogContainer: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
}

func TestLogContainerNotFound(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := "/containers/abc123/logs"
	request, _ := http.NewRequest(http.MethodGet, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("LogContainer: wrong status. Want %d. Got %d.", http.StatusNotFound, recorder.Code)
	}
}

func TestAttachContainer(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	getContainer(&server).State.Running = true
	server.buildMuxer()
	recorder := &HijackableResponseRecorder{}
	path := fmt.Sprintf("/containers/%s/attach?logs=1", getContainer(&server).ID)
	request, _ := http.NewRequest(http.MethodPost, path, nil)
	server.ServeHTTP(recorder, request)
	lines := []string{
		"\x01\x00\x00\x00\x00\x00\x00\x15Container is running",
		"\x01\x00\x00\x00\x00\x00\x00\x0fWhat happened?",
		"\x01\x00\x00\x00\x00\x00\x00\x13Something happened",
	}
	expected := strings.Join(lines, "\n") + "\n"
	if body := recorder.HijackBuffer(); body != expected {
		t.Errorf("AttachContainer: wrong body. Want %q. Got %q.", expected, body)
	}
}

func TestAttachContainerNotFound(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := &HijackableResponseRecorder{}
	path := "/containers/abc123/attach?logs=1"
	request, _ := http.NewRequest(http.MethodPost, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("AttachContainer: wrong status. Want %d. Got %d.", http.StatusNotFound, recorder.Code)
	}
}

func TestAttachContainerWithStreamBlocks(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	getContainer(&server).State.Running = true
	server.buildMuxer()
	path := fmt.Sprintf("/containers/%s/attach?logs=1&stdout=1&stream=1", getContainer(&server).ID)
	request, _ := http.NewRequest(http.MethodPost, path, nil)
	done := make(chan string)
	go func() {
		recorder := &HijackableResponseRecorder{}
		server.ServeHTTP(recorder, request)
		done <- recorder.HijackBuffer()
	}()
	select {
	case <-done:
		t.Fatalf("attach stream returned before container is stopped")
	case <-time.After(500 * time.Millisecond):
	}
	server.cMut.Lock()
	getContainer(&server).State.Running = false
	server.cMut.Unlock()
	var body string
	select {
	case body = <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for attach to finish")
	}
	lines := []string{
		"\x01\x00\x00\x00\x00\x00\x00\x15Container is running",
		"\x01\x00\x00\x00\x00\x00\x00\x0fWhat happened?",
		"\x01\x00\x00\x00\x00\x00\x00\x13Something happened",
	}
	expected := strings.Join(lines, "\n") + "\n"
	if body != expected {
		t.Errorf("AttachContainer: wrong body. Want %q. Got %q.", expected, body)
	}
}

func TestAttachContainerWithStreamBlocksOnCreatedContainers(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	getContainer(&server).State.Running = false
	getContainer(&server).State.StartedAt = time.Time{}
	server.buildMuxer()
	path := fmt.Sprintf("/containers/%s/attach?logs=1&stdout=1&stream=1", getContainer(&server).ID)
	request, _ := http.NewRequest(http.MethodPost, path, nil)
	done := make(chan string)
	go func() {
		recorder := &HijackableResponseRecorder{}
		server.ServeHTTP(recorder, request)
		done <- recorder.HijackBuffer()
	}()
	select {
	case <-done:
		t.Fatalf("attach stream returned before container is stopped")
	case <-time.After(500 * time.Millisecond):
	}
	server.cMut.Lock()
	getContainer(&server).State.StartedAt = time.Now()
	server.cMut.Unlock()
	var body string
	select {
	case body = <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for attach to finish")
	}
	lines := []string{
		"\x01\x00\x00\x00\x00\x00\x00\x19Container is not running",
		"\x01\x00\x00\x00\x00\x00\x00\x0fWhat happened?",
		"\x01\x00\x00\x00\x00\x00\x00\x13Something happened",
	}
	expected := strings.Join(lines, "\n") + "\n"
	if body != expected {
		t.Errorf("AttachContainer: wrong body. Want %q. Got %q.", expected, body)
	}
}

func TestRemoveContainer(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s", getContainer(&server).ID)
	request, _ := http.NewRequest(http.MethodDelete, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Errorf("RemoveContainer: wrong status. Want %d. Got %d.", http.StatusNoContent, recorder.Code)
	}
	if len(server.containers) > 0 {
		t.Error("RemoveContainer: did not remove the container.")
	}
}

func TestRemoveContainerByName(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s", getContainer(&server).Name)
	request, _ := http.NewRequest(http.MethodDelete, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Errorf("RemoveContainer: wrong status. Want %d. Got %d.", http.StatusNoContent, recorder.Code)
	}
	if len(server.containers) > 0 {
		t.Error("RemoveContainer: did not remove the container.")
	}
}

func TestRemoveContainerNotFound(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodDelete, "/containers/abc123", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("RemoveContainer: wrong status. Want %d. Got %d.", http.StatusNotFound, recorder.Code)
	}
}

func TestRemoveContainerRunning(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	getContainer(&server).State.Running = true
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s", getContainer(&server).ID)
	request, _ := http.NewRequest(http.MethodDelete, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("RemoveContainer: wrong status. Want %d. Got %d.", http.StatusInternalServerError, recorder.Code)
	}
	if len(server.containers) < 1 {
		t.Error("RemoveContainer: should not remove the container.")
	}
}

func TestRemoveContainerRunningForce(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	getContainer(&server).State.Running = true
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s?%s", getContainer(&server).ID, "force=1")
	request, _ := http.NewRequest(http.MethodDelete, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Errorf("RemoveContainer: wrong status. Want %d. Got %d.", http.StatusNoContent, recorder.Code)
	}
	if len(server.containers) > 0 {
		t.Error("RemoveContainer: did not remove the container.")
	}
}

func TestPullImage(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPost, "/images/create?fromImage=base", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("PullImage: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	if len(server.images) != 1 {
		t.Errorf("PullImage: Want 1 image. Got %d.", len(server.images))
	}
	if _, ok := server.imgIDs["base"]; !ok {
		t.Error("PullImage: Repository should not be empty.")
	}
	var image docker.Image
	for _, image = range server.images {
	}
	if image.Config == nil {
		t.Error("PullImage: Image Config should not be nil.")
	}
}

func TestPullImageWithTag(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPost, "/images/create?fromImage=base&tag=tag", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("PullImage: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	if len(server.images) != 1 {
		t.Errorf("PullImage: Want 1 image. Got %d.", len(server.images))
	}
	if _, ok := server.imgIDs["base:tag"]; !ok {
		t.Error("PullImage: Repository should not be empty.")
	}
}

func TestPullImageWithShaTag(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPost, "/images/create?fromImage=base&tag=sha256:deadc0de", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("PullImage: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	if len(server.images) != 1 {
		t.Errorf("PullImage: Want 1 image. Got %d.", len(server.images))
	}
	if _, ok := server.imgIDs["base@sha256:deadc0de"]; !ok {
		t.Error("PullImage: Repository should not be empty.")
	}
}

func TestPullImageExisting(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPost, "/images/create?fromImage=base", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("PullImage: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	if len(server.images) != 1 {
		t.Errorf("PullImage: Want 1 image. Got %d.", len(server.images))
	}
	if _, ok := server.imgIDs["base"]; !ok {
		t.Error("PullImage: Repository should not be empty.")
	}
	oldID := server.imgIDs["base"]
	recorder = httptest.NewRecorder()
	request, _ = http.NewRequest(http.MethodPost, "/images/create?fromImage=base", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("PullImage: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	if len(server.images) != 1 {
		t.Errorf("PullImage: Want 1 image. Got %d.", len(server.images))
	}
	if _, ok := server.imgIDs["base"]; !ok {
		t.Error("PullImage: Repository should not be empty.")
	}
	newID := server.imgIDs["base"]
	if oldID != newID {
		t.Error("PullImage: Image ID should be the same after second pull.")
	}
}

func TestPushImage(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.imgIDs = map[string]string{"tsuru/python": "a123"}
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPost, "/images/tsuru/python/push", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("PushImage: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
}

func TestPushImageWithTag(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.imgIDs = map[string]string{"tsuru/python:v1": "a123"}
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPost, "/images/tsuru/python/push?tag=v1", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("PushImage: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
}

func TestPushImageNotFound(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPost, "/images/tsuru/python/push", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("PushImage: wrong status. Want %d. Got %d.", http.StatusNotFound, recorder.Code)
	}
}

func TestTagImage(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.imgIDs = map[string]string{"tsuru/python": "a123"}
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPost, "/images/tsuru/python/tag?repo=tsuru/new-python", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Errorf("TagImage: wrong status. Want %d. Got %d.", http.StatusCreated, recorder.Code)
	}
	if server.imgIDs["tsuru/python"] != server.imgIDs["tsuru/new-python"] {
		t.Errorf("TagImage: did not tag the image")
	}
}

func TestTagImageWithRepoAndTag(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.imgIDs = map[string]string{"tsuru/python": "a123"}
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPost, "/images/tsuru/python/tag?repo=tsuru/new-python&tag=v1", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Errorf("TagImage: wrong status. Want %d. Got %d.", http.StatusCreated, recorder.Code)
	}
	if server.imgIDs["tsuru/python"] != server.imgIDs["tsuru/new-python:v1"] {
		t.Errorf("TagImage: did not tag the image")
	}
}

func TestTagImageWithID(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.images = map[string]docker.Image{"myimgid": {ID: "myimgid"}}
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPost, "/images/myimgid/tag?repo=tsuru/new-python", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Errorf("TagImage: wrong status. Want %d. Got %d.", http.StatusCreated, recorder.Code)
	}
	if server.imgIDs["tsuru/new-python"] != "myimgid" {
		t.Errorf("TagImage: did not tag the image")
	}
}

func TestTagImageNotFound(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPost, "/images/tsuru/python/tag", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("TagImage: wrong status. Want %d. Got %d.", http.StatusNotFound, recorder.Code)
	}
}

func TestInspectImage(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.imgIDs = map[string]string{"tsuru/python": "a123"}
	server.images = map[string]docker.Image{"a123": {ID: "a123", Author: "me"}}
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/images/tsuru/python/json", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("InspectImage: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	var img docker.Image
	err := json.NewDecoder(recorder.Body).Decode(&img)
	if err != nil {
		t.Fatal(err)
	}
	expected := docker.Image{
		ID:     "a123",
		Author: "me",
	}
	if !reflect.DeepEqual(img, expected) {
		t.Errorf("InspectImage: wrong image returned, expected %#v, got: %#v", expected, img)
	}
}

func TestInspectImageWithID(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.images = map[string]docker.Image{"myimgid": {ID: "myimgid", Author: "me"}}
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/images/myimgid/json", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("InspectImage: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	var img docker.Image
	err := json.NewDecoder(recorder.Body).Decode(&img)
	if err != nil {
		t.Fatal(err)
	}
	expected := docker.Image{
		ID:     "myimgid",
		Author: "me",
	}
	if !reflect.DeepEqual(img, expected) {
		t.Errorf("InspectImage: wrong image returned, expected %#v, got: %#v", expected, img)
	}
}

func TestInspectImageNotFound(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/images/tsuru/python/json", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("InspectImage: wrong status. Want %d. Got %d.", http.StatusNotFound, recorder.Code)
	}
}

func addContainers(server *DockerServer, n int) []*docker.Container {
	server.cMut.Lock()
	defer server.cMut.Unlock()
	var addedContainers []*docker.Container
	for i := 0; i < n; i++ {
		date := time.Now().Add(time.Duration((rand.Int() % (i + 1))) * time.Hour)
		container := docker.Container{
			Name:    fmt.Sprintf("%x", rand.Int()%10000),
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
				Image:        "base",
				Labels:       map[string]string{"key": fmt.Sprintf("val-%d", i)},
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
				PortMapping: map[string]docker.PortMapping{
					"Tcp": {"8888": fmt.Sprintf("%d", 49600+i)},
				},
				Ports: map[docker.Port][]docker.PortBinding{
					"8888/tcp": {
						{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", 49600+i)},
					},
				},
			},
			ResolvConfPath: "/etc/resolv.conf",
		}
		server.addContainer(&container)
		addedContainers = append(addedContainers, &container)
	}
	return addedContainers
}

func addImages(server *DockerServer, n int, repo bool) []docker.Image {
	server.iMut.Lock()
	defer server.iMut.Unlock()
	if server.imgIDs == nil {
		server.imgIDs = make(map[string]string)
	}
	if server.images == nil {
		server.images = make(map[string]docker.Image)
	}
	var addedImages []docker.Image
	for i := 0; i < n; i++ {
		date := time.Now().Add(time.Duration((rand.Int() % (i + 1))) * time.Hour)
		image := docker.Image{
			ID:      fmt.Sprintf("%x", rand.Int()%10000),
			Created: date,
		}
		addedImages = append(addedImages, image)
		server.images[image.ID] = image
		if repo {
			repo := "docker/python-" + image.ID
			server.imgIDs[repo] = image.ID
		}
	}
	return addedImages
}

func TestListImages(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addImages(&server, 2, true)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/images/json?all=1", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("ListImages: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	expected := make([]docker.APIImages, 2)
	i := 0
	for _, image := range server.images {
		expected[i] = docker.APIImages{
			ID:       image.ID,
			Created:  image.Created.Unix(),
			RepoTags: []string{"docker/python-" + image.ID},
		}
		i++
	}
	sort.Slice(expected, func(i, j int) bool {
		return expected[i].ID < expected[j].ID
	})
	var got []docker.APIImages
	err := json.NewDecoder(recorder.Body).Decode(&got)
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(got, func(i, j int) bool {
		return got[i].ID < got[j].ID
	})
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("ListImages. Want %#v. Got %#v.", expected, got)
	}
}

func TestRemoveImage(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	images := addImages(&server, 1, false)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/images/%s", images[0].ID)
	request, _ := http.NewRequest(http.MethodDelete, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Errorf("RemoveImage: wrong status. Want %d. Got %d.", http.StatusNoContent, recorder.Code)
	}
	if len(server.images) > 0 {
		t.Error("RemoveImage: did not remove the image.")
	}
}

func TestRemoveImageByName(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	images := addImages(&server, 1, true)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	imgName := "docker/python-" + images[0].ID
	path := "/images/" + imgName
	request, _ := http.NewRequest(http.MethodDelete, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Errorf("RemoveImage: wrong status. Want %d. Got %d.", http.StatusNoContent, recorder.Code)
	}
	if len(server.images) > 0 {
		t.Error("RemoveImage: did not remove the image.")
	}
	_, ok := server.imgIDs[imgName]
	if ok {
		t.Error("RemoveImage: did not remove image tag name.")
	}
}

func TestRemoveImageWithMultipleTags(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	images := addImages(&server, 1, true)
	server.buildMuxer()
	imgID := images[0].ID
	imgName := "docker/python-" + imgID
	server.imgIDs["docker/python-wat"] = imgID
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/images/%s", imgName)
	request, _ := http.NewRequest(http.MethodDelete, path, nil)
	server.ServeHTTP(recorder, request)
	_, ok := server.imgIDs[imgName]
	if ok {
		t.Error("RemoveImage: did not remove image tag name.")
	}
	id, ok := server.imgIDs["docker/python-wat"]
	if !ok {
		t.Error("RemoveImage: removed the wrong tag name.")
	}
	if id != imgID {
		t.Error("RemoveImage: disassociated the wrong ID from the tag")
	}
	if len(server.images) < 1 {
		t.Fatal("RemoveImage: removed the image, but should keep it")
	}
	if server.images[imgID].ID != imgID {
		t.Error("RemoveImage: changed the ID of the image!")
	}
}

func TestRemoveImageByIDWithMultipleTags(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	images := addImages(&server, 1, true)
	server.buildMuxer()
	imgID := images[0].ID
	server.imgIDs["docker/python-wat"] = imgID
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/images/%s", imgID)
	request, _ := http.NewRequest(http.MethodDelete, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Errorf("RemoveImage: wrong status. Want %d. Got %d.", http.StatusConflict, recorder.Code)
	}
}

func TestRemoveImageByIDWithSingleTag(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	images := addImages(&server, 1, true)
	server.buildMuxer()
	imgID := images[0].ID
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/images/%s", imgID)
	request, _ := http.NewRequest(http.MethodDelete, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Errorf("RemoveImage: wrong status. Want %d. Got %d.", http.StatusNoContent, recorder.Code)
	}
	if len(server.images) > 0 {
		t.Error("RemoveImage: did not remove the image.")
	}
	imgName := "docker/python-" + imgID
	_, ok := server.imgIDs[imgName]
	if ok {
		t.Error("RemoveImage: did not remove image tag name.")
	}
}

func TestPrepareFailure(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	errorID := "my_error"
	server.PrepareFailure(errorID, "containers/json")
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/containers/json?all=1", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("PrepareFailure: wrong status. Want %d. Got %d.", http.StatusBadRequest, recorder.Code)
	}
	if recorder.Body.String() != errorID+"\n" {
		t.Errorf("PrepareFailure: wrong message. Want %s. Got %s.", errorID, recorder.Body.String())
	}
}

func TestPrepareMultiFailures(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	errorID := "multi error"
	server.PrepareMultiFailures(errorID, "containers/json")
	server.PrepareMultiFailures(errorID, "containers/json")
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/containers/json?all=1", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("PrepareFailure: wrong status. Want %d. Got %d.", http.StatusBadRequest, recorder.Code)
	}
	if recorder.Body.String() != errorID+"\n" {
		t.Errorf("PrepareFailure: wrong message. Want %s. Got %s.", errorID, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	request, _ = http.NewRequest(http.MethodGet, "/containers/json?all=1", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("PrepareFailure: wrong status. Want %d. Got %d.", http.StatusBadRequest, recorder.Code)
	}
	if recorder.Body.String() != errorID+"\n" {
		t.Errorf("PrepareFailure: wrong message. Want %s. Got %s.", errorID, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	request, _ = http.NewRequest(http.MethodGet, "/containers/json?all=1", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("PrepareFailure: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	if recorder.Body.String() == errorID+"\n" {
		t.Errorf("PrepareFailure: wrong message. Want %s. Got %s.", errorID, recorder.Body.String())
	}
}

func TestRemoveFailure(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	errorID := "my_error"
	server.PrepareFailure(errorID, "containers/json")
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/containers/json?all=1", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("PrepareFailure: wrong status. Want %d. Got %d.", http.StatusBadRequest, recorder.Code)
	}
	server.ResetFailure(errorID)
	recorder = httptest.NewRecorder()
	request, _ = http.NewRequest(http.MethodGet, "/containers/json?all=1", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("RemoveFailure: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
}

func TestResetMultiFailures(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	errorID := "multi error"
	server.PrepareMultiFailures(errorID, "containers/json")
	server.PrepareMultiFailures(errorID, "containers/json")
	if len(server.multiFailures) != 2 {
		t.Errorf("PrepareMultiFailures: error adding multi failures.")
	}
	server.ResetMultiFailures()
	if len(server.multiFailures) != 0 {
		t.Errorf("ResetMultiFailures: error reseting multi failures.")
	}
}

func TestMutateContainer(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	server.addContainer(&docker.Container{ID: "id123"})
	state := docker.State{Running: false, ExitCode: 1}
	err := server.MutateContainer("id123", state)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(getContainer(&server).State, state) {
		t.Errorf("Wrong state after mutation.\nWant %#v.\nGot %#v.",
			state, getContainer(&server).State)
	}
}

func TestMutateContainerNotFound(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	state := docker.State{Running: false, ExitCode: 1}
	err := server.MutateContainer("id123", state)
	if err == nil {
		t.Error("Unexpected <nil> error")
	}
	if err.Error() != "container not found" {
		t.Errorf("wrong error message. Want %q. Got %q.", "container not found", err)
	}
}

func TestBuildImageWithContentTypeTar(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	imageName := "teste"
	recorder := httptest.NewRecorder()
	tarFile, err := os.Open("data/dockerfile.tar")
	if err != nil {
		t.Fatal(err)
	}
	defer tarFile.Close()
	request, _ := http.NewRequest(http.MethodPost, "/build?t=teste", tarFile)
	request.Header.Add("Content-Type", "application/tar")
	server.buildImage(recorder, request)
	if recorder.Body.String() == "miss Dockerfile" {
		t.Errorf("BuildImage: miss Dockerfile")
		return
	}
	if _, ok := server.imgIDs[imageName]; !ok {
		t.Errorf("BuildImage: image %s not builded", imageName)
	}
}

func TestBuildImageWithRemoteDockerfile(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	imageName := "teste"
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPost, "/build?t=teste&remote=http://localhost/Dockerfile", nil)
	server.buildImage(recorder, request)
	if _, ok := server.imgIDs[imageName]; !ok {
		t.Errorf("BuildImage: image %s not builded", imageName)
	}
}

func TestPing(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/_ping", nil)
	server.pingDocker(recorder, request)
	if recorder.Body.String() != "" {
		t.Errorf("Ping: Unexpected body: %s", recorder.Body.String())
	}
	if recorder.Code != http.StatusOK {
		t.Errorf("Ping: Expected code %d, got: %d", http.StatusOK, recorder.Code)
	}
}

func TestDefaultHandler(t *testing.T) {
	t.Parallel()
	server, err := NewServer("127.0.0.1:0", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer server.listener.Close()
	if server.mux != server.DefaultHandler() {
		t.Fatalf("DefaultHandler: Expected to return server.mux, got: %#v", server.DefaultHandler())
	}
}

func TestCreateExecContainer(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	containers := addContainers(&server, 2)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	body := `{"Cmd": ["bash", "-c", "ls"]}`
	path := fmt.Sprintf("/containers/%s/exec", containers[0].ID)
	request, _ := http.NewRequest(http.MethodPost, path, strings.NewReader(body))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("CreateExec: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	serverExec := server.execs[0]
	var got docker.Exec
	err := json.NewDecoder(recorder.Body).Decode(&got)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != serverExec.ID {
		t.Errorf("CreateExec: wrong value. Want %#v. Got %#v.", serverExec.ID, got.ID)
	}

	expected := docker.ExecInspect{
		ID: got.ID,
		ProcessConfig: docker.ExecProcessConfig{
			EntryPoint: "bash",
			Arguments:  []string{"-c", "ls"},
		},
		ContainerID: containers[0].ID,
	}

	if !reflect.DeepEqual(*serverExec, expected) {
		t.Errorf("InspectContainer: wrong value. Want:\n%#v\nGot:\n%#v\n", expected, *serverExec)
	}
}

func TestInspectExecContainer(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addContainers(&server, 1)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	body := `{"Cmd": ["bash", "-c", "ls"]}`
	path := fmt.Sprintf("/containers/%s/exec", getContainer(&server).ID)
	request, _ := http.NewRequest(http.MethodPost, path, strings.NewReader(body))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("CreateExec: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	var got docker.Exec
	err := json.NewDecoder(recorder.Body).Decode(&got)
	if err != nil {
		t.Fatal(err)
	}
	path = fmt.Sprintf("/exec/%s/json", got.ID)
	request, _ = http.NewRequest(http.MethodGet, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("CreateExec: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	var got2 docker.ExecInspect
	err = json.NewDecoder(recorder.Body).Decode(&got2)
	if err != nil {
		t.Fatal(err)
	}
	expected := docker.ExecInspect{
		ID: got.ID,
		ProcessConfig: docker.ExecProcessConfig{
			EntryPoint: "bash",
			Arguments:  []string{"-c", "ls"},
		},
		ContainerID: getContainer(&server).ID,
	}

	if !reflect.DeepEqual(got2, expected) {
		t.Errorf("InspectContainer: wrong value. Want:\n%#v\nGot:\n%#v\n", expected, got2)
	}
}

func TestStartExecContainer(t *testing.T) {
	t.Parallel()
	server, _ := NewServer("127.0.0.1:0", nil, nil)
	defer server.Stop()
	addContainers(server, 1)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	body := `{"Cmd": ["bash", "-c", "ls"]}`
	path := fmt.Sprintf("/containers/%s/exec", getContainer(server).ID)
	request, _ := http.NewRequest(http.MethodPost, path, strings.NewReader(body))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("CreateExec: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	var exec docker.Exec
	err := json.NewDecoder(recorder.Body).Decode(&exec)
	if err != nil {
		t.Fatal(err)
	}
	unleash := make(chan bool)
	server.PrepareExec(exec.ID, func() {
		<-unleash
	})
	codes := make(chan int, 1)
	sent := make(chan bool)
	go func() {
		recorder := httptest.NewRecorder()
		path := fmt.Sprintf("/exec/%s/start", exec.ID)
		body := `{"Tty":true}`
		request, _ := http.NewRequest(http.MethodPost, path, strings.NewReader(body))
		close(sent)
		server.ServeHTTP(recorder, request)
		codes <- recorder.Code
	}()
	<-sent
	execInfo, err := waitExec(server.URL(), exec.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !execInfo.Running {
		t.Error("StartExec: expected exec to be running, but it's not running")
	}
	close(unleash)
	if code := <-codes; code != http.StatusOK {
		t.Errorf("StartExec: wrong status. Want %d. Got %d.", http.StatusOK, code)
	}
	execInfo, err = waitExec(server.URL(), exec.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if execInfo.Running {
		t.Error("StartExec: expected exec to be not running after start returns, but it's running")
	}
}

func TestStartExecContainerWildcardCallback(t *testing.T) {
	t.Parallel()
	server, _ := NewServer("127.0.0.1:0", nil, nil)
	defer server.Stop()
	addContainers(server, 1)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	body := `{"Cmd": ["bash", "-c", "ls"]}`
	path := fmt.Sprintf("/containers/%s/exec", getContainer(server).ID)
	request, _ := http.NewRequest(http.MethodPost, path, strings.NewReader(body))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("CreateExec: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	unleash := make(chan bool)
	server.PrepareExec("*", func() {
		<-unleash
	})
	var exec docker.Exec
	err := json.NewDecoder(recorder.Body).Decode(&exec)
	if err != nil {
		t.Fatal(err)
	}
	codes := make(chan int, 1)
	sent := make(chan bool)
	go func() {
		recorder := httptest.NewRecorder()
		path := fmt.Sprintf("/exec/%s/start", exec.ID)
		body := `{"Tty":true}`
		request, _ := http.NewRequest(http.MethodPost, path, strings.NewReader(body))
		close(sent)
		server.ServeHTTP(recorder, request)
		codes <- recorder.Code
	}()
	<-sent
	execInfo, err := waitExec(server.URL(), exec.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !execInfo.Running {
		t.Error("StartExec: expected exec to be running, but it's not running")
	}
	close(unleash)
	if code := <-codes; code != http.StatusOK {
		t.Errorf("StartExec: wrong status. Want %d. Got %d.", http.StatusOK, code)
	}
	execInfo, err = waitExec(server.URL(), exec.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if execInfo.Running {
		t.Error("StartExec: expected exec to be not running after start returns, but it's running")
	}
}

func TestStartExecContainerNotFound(t *testing.T) {
	t.Parallel()
	server, _ := NewServer("127.0.0.1:0", nil, nil)
	defer server.Stop()
	addContainers(server, 1)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	body := `{"Tty":true}`
	request, _ := http.NewRequest(http.MethodPost, "/exec/something-wat/start", strings.NewReader(body))
	server.ServeHTTP(recorder, request)
}

func waitExec(url, execID string, running bool) (*docker.ExecInspect, error) {
	const maxTry = 5
	client, err := docker.NewClient(url)
	if err != nil {
		return nil, err
	}
	exec, err := client.InspectExec(execID)
	for i := 0; i < maxTry && exec.Running != running && err == nil; i++ {
		time.Sleep(100e6)
		exec, err = client.InspectExec(exec.ID)
	}
	return exec, err
}

func TestStatsContainer(t *testing.T) {
	t.Parallel()
	server, err := NewServer("127.0.0.1:0", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()
	containers := addContainers(server, 2)
	server.buildMuxer()
	expected := docker.Stats{}
	expected.CPUStats.CPUUsage.TotalUsage = 20
	server.PrepareStats(containers[0].ID, func(id string) docker.Stats {
		return expected
	})
	recorder := httptest.NewRecorder()
	path := fmt.Sprintf("/containers/%s/stats?stream=false", containers[0].ID)
	request, _ := http.NewRequest(http.MethodGet, path, nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("StatsContainer: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	body := recorder.Body.Bytes()
	var got docker.Stats
	err = json.Unmarshal(body, &got)
	if err != nil {
		t.Fatal(err)
	}
	got.Read = time.Time{}
	got.PreRead = time.Time{}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("StatsContainer: wrong value. Want %#v. Got %#v.", expected, got)
	}
}

type safeWriter struct {
	sync.Mutex
	*httptest.ResponseRecorder
}

func (w *safeWriter) Write(buf []byte) (int, error) {
	w.Lock()
	defer w.Unlock()
	return w.ResponseRecorder.Write(buf)
}

func TestStatsContainerStream(t *testing.T) {
	t.Parallel()
	server, err := NewServer("127.0.0.1:0", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()
	containers := addContainers(server, 2)
	server.buildMuxer()
	expected := docker.Stats{}
	expected.CPUStats.CPUUsage.TotalUsage = 20
	server.PrepareStats(containers[0].ID, func(string) docker.Stats {
		time.Sleep(50 * time.Millisecond)
		return expected
	})
	recorder := &safeWriter{
		ResponseRecorder: httptest.NewRecorder(),
	}
	path := fmt.Sprintf("/containers/%s/stats?stream=true", containers[0].ID)
	request, _ := http.NewRequest(http.MethodGet, path, nil)
	go func() {
		server.ServeHTTP(recorder, request)
	}()
	time.Sleep(200 * time.Millisecond)
	recorder.Lock()
	defer recorder.Unlock()
	body := recorder.Body.Bytes()
	parts := bytes.Split(body, []byte("\n"))
	if len(parts) < 2 {
		t.Errorf("StatsContainer: wrong number of parts. Want at least 2. Got %#v.", len(parts))
	}
	var got docker.Stats
	err = json.Unmarshal(parts[0], &got)
	if err != nil {
		t.Fatal(err)
	}
	got.Read = time.Time{}
	got.PreRead = time.Time{}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("StatsContainer: wrong value. Want %#v. Got %#v.", expected, got)
	}
}

func addNetworks(server *DockerServer, n int) {
	server.netMut.Lock()
	defer server.netMut.Unlock()
	for i := 0; i < n; i++ {
		netid := fmt.Sprintf("%x", rand.Int()%10000)
		network := docker.Network{
			Name:   netid,
			ID:     fmt.Sprintf("%x", rand.Int()%10000),
			Driver: "bridge",
			Containers: map[string]docker.Endpoint{
				"blah": {
					Name: "blah",
					ID:   fmt.Sprintf("%x", rand.Int()%10000),
				},
			},
		}
		server.networks = append(server.networks, &network)
	}
}

func TestListNetworks(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	addNetworks(&server, 2)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/networks", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("ListNetworks: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	expected := make([]docker.Network, 2)
	for i, network := range server.networks {
		expected[i] = docker.Network{
			ID:         network.ID,
			Name:       network.Name,
			Driver:     network.Driver,
			Containers: network.Containers,
		}
	}
	var got []docker.Network
	err := json.NewDecoder(recorder.Body).Decode(&got)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("ListNetworks. Want %#v. Got %#v.", expected, got)
	}
}

type createNetworkResponse struct {
	ID string `json:"ID"`
}

func TestCreateNetwork(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	netid := fmt.Sprintf("%x", rand.Int()%10000)
	netname := fmt.Sprintf("%x", rand.Int()%10000)
	body := fmt.Sprintf(`{"ID": "%s", "Name": "%s", "Type": "bridge" }`, netid, netname)
	request, _ := http.NewRequest(http.MethodPost, "/networks/create", strings.NewReader(body))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Errorf("CreateNetwork: wrong status. Want %d. Got %d.", http.StatusCreated, recorder.Code)
	}

	var returned createNetworkResponse
	err := json.NewDecoder(recorder.Body).Decode(&returned)
	if err != nil {
		t.Fatal(err)
	}
	stored := server.networks[0]
	if returned.ID != stored.ID {
		t.Errorf("CreateNetwork: ID mismatch. Stored: %q. Returned: %q.", stored.ID, returned)
	}
}

func TestCreateNetworkInvalidBody(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPost, "/networks/create", strings.NewReader("whaaaaaat---"))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("CreateNetwork: wrong status. Want %d. Got %d.", http.StatusBadRequest, recorder.Code)
	}
}

func TestCreateNetworkDuplicateName(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	addNetworks(&server, 1)
	server.networks[0].Name = "mynetwork"
	recorder := httptest.NewRecorder()
	body := fmt.Sprintf(`{"ID": "%s", "Name": "mynetwork", "Type": "bridge" }`, fmt.Sprintf("%x", rand.Int()%10000))
	request, _ := http.NewRequest(http.MethodPost, "/networks/create", strings.NewReader(body))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Errorf("CreateNetwork: wrong status. Want %d. Got %d.", http.StatusForbidden, recorder.Code)
	}
}

func TestRemoveNetwork(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	server.networks = []*docker.Network{
		{ID: "id1", Name: "name1"},
		{ID: "id2", Name: "name2"},
	}
	request, _ := http.NewRequest(http.MethodDelete, "/networks/id1", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Errorf("RemoveNetwork: wrong status. Want %d. Got %d.", http.StatusNoContent, recorder.Code)
	}
	expected := []*docker.Network{{ID: "id2", Name: "name2"}}
	if !reflect.DeepEqual(server.networks, expected) {
		t.Errorf("RemoveNetwork: expected networks to be %#v, got %#v", expected, server.networks)
	}
	request, _ = http.NewRequest(http.MethodDelete, "/networks/name2", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Errorf("RemoveNetwork: wrong status. Want %d. Got %d.", http.StatusNoContent, recorder.Code)
	}
	expected = []*docker.Network{}
	if !reflect.DeepEqual(server.networks, expected) {
		t.Errorf("RemoveNetwork: expected networks to be %#v, got %#v", expected, server.networks)
	}
}

func TestNetworkConnect(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	addNetworks(&server, 1)
	server.networks[0].ID = fmt.Sprintf("%x", rand.Int()%10000)
	server.imgIDs = map[string]string{"base": "a1234"}
	containers := addContainers(&server, 1)
	containers[0].ID = fmt.Sprintf("%x", rand.Int()%10000)
	server.addContainer(containers[0])

	recorder := httptest.NewRecorder()
	body := fmt.Sprintf(`{"Container": "%s" }`, containers[0].ID)
	request, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/networks/%s/connect", server.networks[0].ID), strings.NewReader(body))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("NetworkConnect: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
}

func TestListVolumes(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	expected := []docker.Volume{{
		Name:       "test-vol-1",
		Driver:     "local",
		Mountpoint: "/var/lib/docker/volumes/test-vol-1",
	}}
	server.volStore = make(map[string]*volumeCounter)
	for _, vol := range expected {
		server.volStore[vol.Name] = &volumeCounter{
			volume: vol,
			count:  0,
		}
	}
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/volumes", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("ListVolumes: wrong status.  Want %d. Got %d.", http.StatusCreated, recorder.Code)
	}
	var got map[string][]docker.Volume
	err := json.NewDecoder(recorder.Body).Decode(&got)
	if err != nil {
		t.Fatal(err)
	}

	gotVolumes, ok := got["Volumes"]
	if !ok {
		t.Fatal("ListVolumes failed can not find Volumes")
	}
	if !reflect.DeepEqual(gotVolumes, expected) {
		t.Errorf("ListVolumes.  Want %#v.  Got %#v.", expected, got)
	}
}

func TestCreateVolume(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	body := `{"Name":"test-volume"}`
	request, _ := http.NewRequest(http.MethodPost, "/volumes/create", strings.NewReader(body))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Errorf("CreateVolume: wrong status.  Want %d. Got %d.", http.StatusCreated, recorder.Code)
	}
	var returned docker.Volume
	err := json.NewDecoder(recorder.Body).Decode(&returned)
	if err != nil {
		t.Error(err)
	}
	if returned.Name != "test-volume" {
		t.Errorf("CreateVolume: Name mismatch.  Expected: test-volume.  Returned %q.", returned.Name)
	}
	if returned.Driver != "local" {
		t.Errorf("CreateVolume: Driver mismatch.  Expected: local.  Returned: %q", returned.Driver)
	}
	if returned.Mountpoint != "/var/lib/docker/volumes/test-volume" {
		t.Errorf("CreateVolume:  Mountpoint mismatch.  Expected: /var/lib/docker/volumes/test-volume.  Returned: %q.", returned.Mountpoint)
	}
}

func TestCreateVolumeAlreadExists(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	server.volStore = make(map[string]*volumeCounter)
	server.volStore["test-volume"] = &volumeCounter{
		volume: docker.Volume{
			Name:       "test-volume",
			Driver:     "local",
			Mountpoint: "/var/lib/docker/volumes/test-volume",
		},
		count: 0,
	}
	body := `{"Name":"test-volume"}`
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPost, "/volumes/create", strings.NewReader(body))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Errorf("CreateVolumeAlreadExists: wrong status.  Want %d. Got %d.", http.StatusCreated, recorder.Code)
	}
	var returned docker.Volume
	err := json.NewDecoder(recorder.Body).Decode(&returned)
	if err != nil {
		t.Error(err)
	}
	if returned.Name != "test-volume" {
		t.Errorf("CreateVolumeAlreadExists: Name mismatch.  Expected: test-volume.  Returned %q.", returned.Name)
	}
	if returned.Driver != "local" {
		t.Errorf("CreateVolumeAlreadExists: Driver mismatch.  Expected: local.  Returned: %q", returned.Driver)
	}
	if returned.Mountpoint != "/var/lib/docker/volumes/test-volume" {
		t.Errorf("CreateVolumeAlreadExists:  Mountpoint mismatch.  Expected: /var/lib/docker/volumes/test-volume.  Returned: %q.", returned.Mountpoint)
	}
}

func TestInspectVolume(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	expected := docker.Volume{
		Name:       "test-volume",
		Driver:     "local",
		Mountpoint: "/var/lib/docker/volumes/test-volume",
	}
	volC := &volumeCounter{
		volume: expected,
		count:  0,
	}
	volStore := make(map[string]*volumeCounter)
	volStore["test-volume"] = volC
	server.volStore = volStore
	request, _ := http.NewRequest(http.MethodGet, "/volumes/test-volume", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("InspectVolume: wrong status.  Want %d.  God %d.", http.StatusOK, recorder.Code)
	}
	var returned docker.Volume
	err := json.NewDecoder(recorder.Body).Decode(&returned)
	if err != nil {
		t.Error(err)
	}
	if returned.Name != "test-volume" {
		t.Errorf("InspectVolume: Name mismatch.  Expected: test-volume.  Returned %q.", returned.Name)
	}
	if returned.Driver != "local" {
		t.Errorf("InspectVolume: Driver mismatch.  Expected: local.  Returned: %q", returned.Driver)
	}
	if returned.Mountpoint != "/var/lib/docker/volumes/test-volume" {
		t.Errorf("InspectVolume:  Mountpoint mismatch.  Expected: /var/lib/docker/volumes/test-volume.  Returned: %q.", returned.Mountpoint)
	}
}

func TestInspectVolumeNotFound(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/volumes/test-volume", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("RemoveMissingVolume: wrong status.  Want %d.  Got %d.", http.StatusNotFound, recorder.Code)
	}
}

func TestRemoveVolume(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	server.volStore = make(map[string]*volumeCounter)
	server.volStore["test-volume"] = &volumeCounter{
		volume: docker.Volume{
			Name:       "test-volume",
			Driver:     "local",
			Mountpoint: "/var/lib/docker/volumes/test-volume",
		},
		count: 0,
	}
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodDelete, "/volumes/test-volume", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Errorf("RemoveVolume: wrong status.  Want %d.  Got %d.", http.StatusNoContent, recorder.Code)
	}
}

func TestRemoveMissingVolume(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodDelete, "/volumes/test-volume", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("RemoveMissingVolume: wrong status.  Want %d.  Got %d.", http.StatusNotFound, recorder.Code)
	}
}

func TestRemoveVolumeInuse(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	server.volStore = make(map[string]*volumeCounter)
	server.volStore["test-volume"] = &volumeCounter{
		volume: docker.Volume{
			Name:       "test-volume",
			Driver:     "local",
			Mountpoint: "/var/lib/docker/volumes/test-volume",
		},
		count: 1,
	}
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodDelete, "/volumes/test-volume", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Errorf("RemoveVolume: wrong status.  Want %d.  Got %d.", http.StatusConflict, recorder.Code)
	}
}

func TestUploadToContainer(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	cont := &docker.Container{
		ID: "id123",
		State: docker.State{
			Running:  true,
			ExitCode: 0,
		},
	}
	server.addContainer(cont)
	server.uploadedFiles = make(map[string]string)
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("/containers/%s/archive?path=abcd", cont.ID), nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("UploadToContainer: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	if val, ok := server.uploadedFiles[cont.ID]; !ok {
		t.Errorf("UploadToContainer: uploadedFiles should exist.")
	} else if val != "abcd" {
		t.Errorf("UploadToContainer: wrong uploadedFiles. Want 'abcd'. Got %s.", val)
	}
}

func TestUploadToContainerWithBodyTarFile(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	cont := &docker.Container{
		ID: "id123",
		State: docker.State{
			Running:  true,
			ExitCode: 0,
		},
	}
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer tw.Close()
	hdr := &tar.Header{
		Name: "test.tar.gz",
		Mode: 0o600,
		Size: int64(buf.Len()),
	}
	tw.WriteHeader(hdr)
	tw.Write([]byte("something"))
	tw.Close()
	server.addContainer(cont)
	server.uploadedFiles = make(map[string]string)
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("/containers/%s/archive?path=abcd", cont.ID), buf)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("UploadToContainer: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	if val, ok := server.uploadedFiles[cont.ID]; !ok {
		t.Errorf("UploadToContainer: uploadedFiles should exist.")
	} else if val != "abcd/test.tar.gz" {
		t.Errorf("UploadToContainer: wrong uploadedFiles. Want 'abcd/test.tar.gz'. Got %s.", val)
	}
}

func TestUploadToContainerBodyNotTarFile(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	cont := &docker.Container{
		ID: "id123",
		State: docker.State{
			Running:  true,
			ExitCode: 0,
		},
	}
	buf := bytes.NewBufferString("something")
	server.addContainer(cont)
	server.uploadedFiles = make(map[string]string)
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("/containers/%s/archive?path=abcd", cont.ID), buf)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("UploadToContainer: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	if val, ok := server.uploadedFiles[cont.ID]; !ok {
		t.Errorf("UploadToContainer: uploadedFiles should exist.")
	} else if val != "abcd" {
		t.Errorf("UploadToContainer: wrong uploadedFiles. Want 'abcd'. Got %s.", val)
	}
}

func TestUploadToContainerMissingContainer(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodPut, "/containers/missing-container/archive?path=abcd", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("UploadToContainer: wrong status. Want %d. Got %d.", http.StatusNotFound, recorder.Code)
	}
}

func TestInfoDocker(t *testing.T) {
	t.Parallel()
	server, _ := NewServer("127.0.0.1:0", nil, nil)
	defer server.Stop()
	addContainers(server, 1)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/info", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("InfoDocker: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	var infoData map[string]any
	err := json.Unmarshal(recorder.Body.Bytes(), &infoData)
	if err != nil {
		t.Fatal(err)
	}
	if infoData["Containers"].(float64) != 1.0 {
		t.Fatalf("InfoDocker: wrong containers count. Want %f. Got %f.", 1.0, infoData["Containers"])
	}
	if infoData["DockerRootDir"].(string) != "/var/lib/docker" {
		t.Fatalf("InfoDocker: wrong docker root. Want /var/lib/docker. Got %s.", infoData["DockerRootDir"])
	}
}

func TestInfoDockerWithSwarm(t *testing.T) {
	t.Parallel()
	srv1, srv2 := setUpSwarm(t)
	defer srv1.Stop()
	defer srv2.Stop()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/info", nil)
	srv1.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("InfoDocker: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	var infoData docker.DockerInfo
	err := json.Unmarshal(recorder.Body.Bytes(), &infoData)
	if err != nil {
		t.Fatal(err)
	}
	expectedSwarm := swarm.Info{
		NodeID: srv1.nodeID,
		RemoteManagers: []swarm.Peer{
			{NodeID: srv1.nodeID, Addr: srv1.SwarmAddress()},
			{NodeID: srv2.nodeID, Addr: srv2.SwarmAddress()},
		},
	}
	if !reflect.DeepEqual(infoData.Swarm, expectedSwarm) {
		t.Fatalf("InfoDocker: wrong swarm info. Want:\n%#v\nGot:\n%#v", expectedSwarm, infoData.Swarm)
	}
}

func TestVersionDocker(t *testing.T) {
	t.Parallel()
	server, _ := NewServer("127.0.0.1:0", nil, nil)
	defer server.Stop()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/version", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("VersionDocker: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
}

func TestDownloadFromContainer(t *testing.T) {
	t.Parallel()
	server := baseDockerServer()
	server.buildMuxer()
	cont := &docker.Container{
		ID: "id123",
		State: docker.State{
			Running:  true,
			ExitCode: 0,
		},
	}
	server.addContainer(cont)
	server.uploadedFiles = make(map[string]string)
	server.uploadedFiles[cont.ID] = "abcd"
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/containers/%s/archive?path=abcd", cont.ID), nil)
	server.ServeHTTP(recorder, request)
	resp := recorder.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("DownloadFromContainer: wrong status. Want %d. Got %d.", http.StatusOK, resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "application/x-tar" {
		t.Errorf("DownloadFromContainer: wrong Content-Type. Want 'application/x-tar'. Got %s.", resp.Header.Get("Content-Type"))
	}
}

func TestSupportVersionPathPrefix(t *testing.T) {
	t.Parallel()
	server, _ := NewServer("127.0.0.1:0", nil, nil)
	defer server.Stop()
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest(http.MethodGet, "/v1.16/version", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("VersionDocker: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
}
