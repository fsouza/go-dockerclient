// Copyright 2013 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package testing provides a fake implementation of the Docker API, useful for
// testing purpose.
package testing

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/bmizerany/pat"
	"github.com/dotcloud/docker"
	mathrand "math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DockerServer represents a programmable, concurrent, HTTP server implementing
// a fake version of the Docker remote API.
//
// It can used in standalone mode, listening for connections or as an arbitrary
// HTTP handler.
//
// For more details on the remote API, check http://goo.gl/yMI1S.
type DockerServer struct {
	containers []docker.Container
	cMut       sync.RWMutex
	images     []docker.Image
	iMut       sync.RWMutex
	imgIDs     map[string]string
	listener   net.Listener
	mux        *pat.PatternServeMux
}

// NewServer returns a new instance of the fake server, in standalone mode. Use
// the method URL to get the URL of the server.
func NewServer() (*DockerServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	server := DockerServer{listener: listener}
	server.buildMuxer()
	go http.Serve(listener, &server)
	return &server, nil
}

func (s *DockerServer) buildMuxer() {
	s.mux = pat.New()
	s.mux.Post("/:version/commit", http.HandlerFunc(s.commitContainer))
	s.mux.Get("/:version/containers/json", http.HandlerFunc(s.listContainers))
	s.mux.Post("/:version/containers/create", http.HandlerFunc(s.createContainer))
	s.mux.Get("/:version/containers/:id/json", http.HandlerFunc(s.inspectContainer))
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
}

func (s *DockerServer) listContainers(w http.ResponseWriter, r *http.Request) {
	s.cMut.RLock()
	result := make([]docker.APIContainers, len(s.containers))
	for i, container := range s.containers {
		result[i] = docker.APIContainers{
			ID:      container.ID,
			Image:   container.Image,
			Command: fmt.Sprintf("%s %s", container.Path, strings.Join(container.Args, " ")),
			Created: container.Created.Unix(),
			Status:  container.State.String(),
			Ports:   container.NetworkSettings.PortMappingHuman(),
		}
	}
	s.cMut.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (s *DockerServer) createContainer(w http.ResponseWriter, r *http.Request) {
	var config docker.Config
	defer r.Body.Close()
	err := json.NewDecoder(r.Body).Decode(&config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.iMut.RLock()
	image, ok := s.imgIDs[config.Image]
	s.iMut.RUnlock()
	if !ok {
		http.Error(w, "No such image", http.StatusNotFound)
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
			Running:   true,
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
	s.containers = append(s.containers, container)
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
	id := r.URL.Query().Get(":id")
	var container docker.Container
	index := -1
	s.cMut.RLock()
	for i, c := range s.containers {
		if c.ID == id {
			container = c
			index = i
			break
		}
	}
	s.cMut.RUnlock()
	if index < 0 {
		http.Error(w, "No such container", http.StatusNotFound)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(container)
}

func (s *DockerServer) commitContainer(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("container")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"ID":%q}`, id)
}
