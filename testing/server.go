// Copyright 2013 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package testing provides a fake implementation of the Docker API, useful for
// testing purpose.
package testing

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dotcloud/docker"
	"github.com/gorilla/mux"
	mathrand "math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DockerServer represents a programmable, concurrent (not much), HTTP server
// implementing a fake version of the Docker remote API.
//
// It can used in standalone mode, listening for connections or as an arbitrary
// HTTP handler.
//
// For more details on the remote API, check http://goo.gl/yMI1S.
type DockerServer struct {
	containers []*docker.Container
	cMut       sync.RWMutex
	images     []docker.Image
	iMut       sync.RWMutex
	imgIDs     map[string]string
	listener   net.Listener
	mux        *mux.Router
	hook       func(*http.Request)
}

// NewServer returns a new instance of the fake server, in standalone mode. Use
// the method URL to get the URL of the server.
//
// Hook is a function that will be called on every request.
func NewServer(hook func(*http.Request)) (*DockerServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	server := DockerServer{listener: listener, imgIDs: make(map[string]string), hook: hook}
	server.buildMuxer()
	go http.Serve(listener, &server)
	return &server, nil
}

func (s *DockerServer) buildMuxer() {
	s.mux = mux.NewRouter()
	s.mux.Path("/v{version:[0-9.]+}/commit").Methods("POST").HandlerFunc(s.commitContainer)
	s.mux.Path("/v{version:[0-9.]+}/containers/json").Methods("GET").HandlerFunc(s.listContainers)
	s.mux.Path("/v{version:[0-9.]+}/containers/create").Methods("POST").HandlerFunc(s.createContainer)
	s.mux.Path("/v{version:[0-9.]+}/containers/{id:.*}/json").Methods("GET").HandlerFunc(s.inspectContainer)
	s.mux.Path("/v{version:[0-9.]+}/containers/{id:.*}/start").Methods("POST").HandlerFunc(s.startContainer)
	s.mux.Path("/v{version:[0-9.]+}/containers/{id:.*}/stop").Methods("POST").HandlerFunc(s.stopContainer)
	s.mux.Path("/v{version:[0-9.]+}/containers/{id:.*}/wait").Methods("POST").HandlerFunc(s.waitContainer)
	s.mux.Path("/v{version:[0-9.]+}/containers/{id:.*}/attach").Methods("POST").HandlerFunc(s.attachContainer)
	s.mux.Path("/v{version:[0-9.]+}/containers/{id:.*}").Methods("DELETE").HandlerFunc(s.removeContainer)
	s.mux.Path("/v{version:[0-9.]+}/images/create").Methods("POST").HandlerFunc(s.pullImage)
	s.mux.Path("/v{version:[0-9.]+}/images/json").Methods("GET").HandlerFunc(s.listImages)
	s.mux.Path("/v{version:[0-9.]+}/images/{id:.*}").Methods("DELETE").HandlerFunc(s.removeImage)
	s.mux.Path("/v{version:[0-9.]+}/images/{name:.*}/push").Methods("POST").HandlerFunc(s.pushImage)
}

// Stop stops the server.
func (s *DockerServer) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
}

// URL returns the HTTP URL of the server.
func (s *DockerServer) URL() string {
	if s.listener == nil {
		return ""
	}
	return "http://" + s.listener.Addr().String() + "/"
}

// ServeHTTP handles HTTP requests sent to the server.
func (s *DockerServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
	if s.hook != nil {
		s.hook(r)
	}
}

func (s *DockerServer) listContainers(w http.ResponseWriter, r *http.Request) {
	all := r.URL.Query().Get("all")
	s.cMut.RLock()
	result := make([]docker.APIContainers, len(s.containers))
	for i, container := range s.containers {
		if all == "1" || container.State.Running == true {
			result[i] = docker.APIContainers{
				ID:      container.ID,
				Image:   container.Image,
				Command: fmt.Sprintf("%s %s", container.Path, strings.Join(container.Args, " ")),
				Created: container.Created.Unix(),
				Status:  container.State.String(),
				Ports:   container.NetworkSettings.PortMappingHuman(),
			}
		}
	}
	s.cMut.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (s *DockerServer) listImages(w http.ResponseWriter, r *http.Request) {
	s.cMut.RLock()
	result := make([]docker.APIImages, len(s.images))
	for i, image := range s.images {
		result[i] = docker.APIImages{
			ID:      image.ID,
			Created: image.Created.Unix(),
		}
	}
	s.cMut.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (s *DockerServer) findImage(id string) (string, error) {
	s.iMut.RLock()
	defer s.iMut.RUnlock()
	image, ok := s.imgIDs[id]
	if ok {
		return image, nil
	}
	image, _, err := s.findImageByID(id)
	return image, err
}

func (s *DockerServer) findImageByID(id string) (string, int, error) {
	s.iMut.RLock()
	defer s.iMut.RUnlock()
	for i, image := range s.images {
		if image.ID == id {
			return image.ID, i, nil
		}
	}
	return "", -1, errors.New("No such image")
}

func (s *DockerServer) createContainer(w http.ResponseWriter, r *http.Request) {
	var config docker.Config
	defer r.Body.Close()
	err := json.NewDecoder(r.Body).Decode(&config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	image, err := s.findImage(config.Image)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusCreated)
	portMapping := make(map[string]string, len(config.PortSpecs))
	for _, p := range config.PortSpecs {
		portMapping[p] = strconv.Itoa(mathrand.Int() % 65536)
	}
	container := docker.Container{
		ID:      s.generateID(),
		Created: time.Now(),
		Path:    config.Cmd[0],
		Args:    config.Cmd[1:],
		Config:  &config,
		State: docker.State{
			Running:   false,
			Pid:       mathrand.Int() % 50000,
			ExitCode:  0,
			StartedAt: time.Now(),
		},
		Image: image,
		NetworkSettings: &docker.NetworkSettings{
			IPAddress:   fmt.Sprintf("172.16.42.%d", mathrand.Int()%250+2),
			IPPrefixLen: 24,
			Gateway:     "172.16.42.1",
			Bridge:      "docker0",
			PortMapping: portMapping,
		},
	}
	s.cMut.Lock()
	s.containers = append(s.containers, &container)
	s.cMut.Unlock()
	var c = struct{ ID string }{ID: container.ID}
	json.NewEncoder(w).Encode(c)
}

func (s *DockerServer) generateID() string {
	var buf [16]byte
	rand.Read(buf[:])
	return fmt.Sprintf("%x", buf)
}

func (s *DockerServer) inspectContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	container, _, err := s.findContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(container)
}

func (s *DockerServer) startContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	container, _, err := s.findContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	s.cMut.Lock()
	defer s.cMut.Unlock()
	if container.State.Running {
		http.Error(w, "Container already running", http.StatusBadRequest)
		return
	}
	container.State.Running = true
}

func (s *DockerServer) stopContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	container, _, err := s.findContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	s.cMut.Lock()
	defer s.cMut.Unlock()
	if !container.State.Running {
		http.Error(w, "Container not running", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	container.State.Running = false
}

func (s *DockerServer) attachContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	container, _, err := s.findContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if container.State.Running {
		fmt.Fprintf(w, "Container %q is running\n", container.ID)
	} else {
		fmt.Fprintf(w, "Container %q is not running\n", container.ID)
	}
	fmt.Fprintln(w, "What happened?")
	fmt.Fprintln(w, "Something happened")
}

func (s *DockerServer) waitContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	container, _, err := s.findContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	for {
		s.cMut.RLock()
		if container.State.Running {
			s.cMut.RUnlock()
			break
		}
		s.cMut.RUnlock()
	}
	w.Write([]byte(`{"StatusCode":0}`))
}

func (s *DockerServer) removeContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	_, index, err := s.findContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if s.containers[index].State.Running {
		msg := "Error: API error (406): Impossible to remove a running container, please stop it first"
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	s.cMut.Lock()
	defer s.cMut.Unlock()
	s.containers[index] = s.containers[len(s.containers)-1]
	s.containers = s.containers[:len(s.containers)-1]
}

func (s *DockerServer) commitContainer(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("container")
	container, _, err := s.findContainer(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	var config *docker.Config
	runConfig := r.URL.Query().Get("run")
	if runConfig != "" {
		config = new(docker.Config)
		err = json.Unmarshal([]byte(runConfig), config)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	image := docker.Image{
		ID:        "img-" + container.ID,
		Parent:    container.Image,
		Container: container.ID,
		Comment:   r.URL.Query().Get("m"),
		Author:    r.URL.Query().Get("author"),
		Config:    config,
	}
	repository := r.URL.Query().Get("repo")
	s.iMut.Lock()
	s.images = append(s.images, image)
	if repository != "" {
		s.imgIDs[repository] = image.ID
	}
	s.iMut.Unlock()
	fmt.Fprintf(w, `{"ID":%q}`, image.ID)
}

func (s *DockerServer) findContainer(id string) (*docker.Container, int, error) {
	s.cMut.RLock()
	defer s.cMut.RUnlock()
	for i, container := range s.containers {
		if container.ID == id {
			return container, i, nil
		}
	}
	return nil, -1, errors.New("No such container")
}

func (s *DockerServer) pullImage(w http.ResponseWriter, r *http.Request) {
	repository := r.URL.Query().Get("fromImage")
	image := docker.Image{
		ID: s.generateID(),
	}
	s.iMut.Lock()
	s.images = append(s.images, image)
	if repository != "" {
		s.imgIDs[repository] = image.ID
	}
	s.iMut.Unlock()
}

func (s *DockerServer) pushImage(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	s.iMut.RLock()
	if _, ok := s.imgIDs[name]; !ok {
		s.iMut.RUnlock()
		http.Error(w, "No such image", http.StatusNotFound)
		return
	}
	s.iMut.RUnlock()
	fmt.Fprintln(w, "Pushing...")
	fmt.Fprintln(w, "Pushed")
}

func (s *DockerServer) removeImage(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	_, index, err := s.findImageByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	s.iMut.Lock()
	defer s.iMut.Unlock()
	s.images[index] = s.images[len(s.images)-1]
	s.images = s.images[:len(s.images)-1]
}
