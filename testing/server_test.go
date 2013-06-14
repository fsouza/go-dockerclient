// Copyright 2013 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testing

import (
	"net"
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
