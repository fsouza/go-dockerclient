// Copyright 2014 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
)

var fakeServer = struct {
	handler     func(w http.ResponseWriter, r *http.Request)
	lastRequest *http.Request
	*httptest.Server
}{}

func init() {
	fakeServer.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body bytes.Buffer
		io.Copy(&body, r.Body)
		r.Body = ioutil.NopCloser(&body)
		copyReq := *r
		copyReq.Body = ioutil.NopCloser(bytes.NewBuffer(body.Bytes()))
		fakeServer.lastRequest = &copyReq
		if fakeServer.handler != nil {
			body.Reset()
			fakeServer.handler(w, r)
		}
	}))
}

func resetFakeServer() {
	fakeServer.handler = nil
}

func statusHandler(status int, message string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(message))
	}
}

func jsonHandler(content string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(content))
	}
}
