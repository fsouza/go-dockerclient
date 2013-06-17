// Copyright 2013 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package testing provides a fake implementation of the Docker API, useful for
// testing purpose.
package testing

import (
	"encoding/json"
	"fmt"
	"github.com/bmizerany/pat"
	"github.com/dotcloud/docker"
	"net"
	"net/http"
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

func (s *DockerServer) commitContainer(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("container")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"ID":%q}`, id)
}

func (s *DockerServer) inspectContainer(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get(":id")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	container := docker.Container{
		ID:      id,
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
	json.NewEncoder(w).Encode(container)
}
