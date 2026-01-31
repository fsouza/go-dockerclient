// Copyright 2013 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testing

import (
	"encoding/binary"
	"io"

	"github.com/moby/moby/api/pkg/stdcopy"
)

// stdWriter wraps an io.Writer to multiplex writes with a header prefix.
// This enables multiple streams (stdout, stderr) to be muxed into a single connection.
type stdWriter struct {
	io.Writer
	prefix byte
}

// newStdWriter creates a new stdWriter that writes to w with the given stream type prefix.
func newStdWriter(w io.Writer, t stdcopy.StdType) io.Writer {
	return &stdWriter{
		Writer: w,
		prefix: byte(t),
	}
}

func (w *stdWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	header := make([]byte, 8)
	header[0] = w.prefix
	binary.BigEndian.PutUint32(header[4:], uint32(len(p)))
	_, err := w.Writer.Write(header)
	if err != nil {
		return 0, err
	}
	return w.Writer.Write(p)
}
