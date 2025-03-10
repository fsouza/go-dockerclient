// Copyright 2017 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !windows

package docker

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/fsouza/go-dockerclient/internal/testutils"
)

func TestClientDoConcurrentStress(t *testing.T) {
	t.Parallel()
	var reqs []*http.Request
	var mu sync.Mutex
	handler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		mu.Lock()
		reqs = append(reqs, r)
		mu.Unlock()
	})
	var nativeSrvs []*httptest.Server
	for range 3 {
		srv, cleanup, err := newNativeServer(handler)
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		nativeSrvs = append(nativeSrvs, srv)
	}
	tests := []struct {
		testCase      string
		srv           *httptest.Server
		scheme        string
		withTimeout   bool
		withTLSServer bool
		withTLSClient bool
	}{
		{testCase: "http server", srv: httptest.NewUnstartedServer(handler), scheme: "http"},
		{testCase: "native server", srv: nativeSrvs[0], scheme: nativeProtocol},
		{testCase: "http with timeout", srv: httptest.NewUnstartedServer(handler), scheme: "http", withTimeout: true},
		{testCase: "native with timeout", srv: nativeSrvs[1], scheme: nativeProtocol, withTimeout: true},
		{testCase: "http with tls", srv: httptest.NewUnstartedServer(handler), scheme: "https", withTLSServer: true, withTLSClient: true},
		{testCase: "native with client-only tls", srv: nativeSrvs[2], scheme: nativeProtocol, withTLSServer: false, withTLSClient: nativeProtocol == unixProtocol}, // TLS client only works with unix protocol
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.testCase, func(t *testing.T) {
			_, serverCert := testutils.GenCertificate(t)

			reqs = nil
			var client *Client
			var err error
			endpoint := tt.scheme + "://" + tt.srv.Listener.Addr().String()
			if tt.withTLSServer {
				tt.srv.StartTLS()
			} else {
				tt.srv.Start()
			}
			defer tt.srv.Close()
			if tt.withTLSClient {
				certPEMBlock, certErr := os.ReadFile(serverCert.CertPath)
				if certErr != nil {
					t.Fatal(certErr)
				}
				keyPEMBlock, certErr := os.ReadFile(serverCert.KeyPath)
				if certErr != nil {
					t.Fatal(certErr)
				}
				client, err = NewTLSClientFromBytes(endpoint, certPEMBlock, keyPEMBlock, nil)
			} else {
				client, err = NewClient(endpoint)
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.withTimeout {
				client.SetTimeout(time.Minute)
			}
			n := 50
			wg := sync.WaitGroup{}
			var paths []string
			errsCh := make(chan error, 3*n)
			waiters := make(chan CloseWaiter, n)
			for i := range n {
				path := fmt.Sprintf("/%05d", i)
				paths = append(paths, http.MethodGet+path)
				paths = append(paths, http.MethodPost+path)
				paths = append(paths, "HEAD"+path)
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, clientErr := client.do(http.MethodGet, path, doOptions{})
					if clientErr != nil {
						errsCh <- clientErr
					}
					clientErr = client.stream(http.MethodPost, path, streamOptions{})
					if clientErr != nil {
						errsCh <- clientErr
					}
					cw, clientErr := client.hijack("HEAD", path, hijackOptions{})
					if clientErr != nil {
						errsCh <- clientErr
					} else {
						waiters <- cw
					}
				}()
			}
			wg.Wait()
			close(errsCh)
			close(waiters)
			for cw := range waiters {
				cw.Wait()
				cw.Close()
			}
			for err = range errsCh {
				t.Error(err)
			}
			var reqPaths []string
			for _, r := range reqs {
				reqPaths = append(reqPaths, r.Method+r.URL.Path)
			}
			slices.Sort(paths)
			slices.Sort(reqPaths)
			if !reflect.DeepEqual(reqPaths, paths) {
				t.Fatalf("expected server request paths to equal %v, got: %v", paths, reqPaths)
			}
		})
	}
}
