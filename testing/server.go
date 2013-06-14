// Copyright 2013 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package testing provides a fake implementation of the Docker API, useful for
// testing purpose.
package testing

import (
	"fmt"
	"github.com/bmizerany/pat"
	"github.com/dotcloud/docker"
	"net"
	"net/http"
	"sync"
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

func (s *DockerServer) commitContainer(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("container")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"ID":%q}`, id)
}

func (s *DockerServer) inspectContainer(w http.ResponseWriter, r *http.Request) {
}
