// Copyright 2026 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Hold Close open after it unblocks Read. This forces the input copier to
// publish its error before Close returns, without relying on scheduler timing.
type execClosingInput struct {
	*io.PipeReader
	started chan struct{}
	once    sync.Once
	closing chan struct{}
	release chan struct{}
	closed  chan struct{}
	closes  atomic.Int32
}

func newExecClosingInput(t *testing.T) *execClosingInput {
	t.Helper()
	r, w := io.Pipe()
	input := &execClosingInput{PipeReader: r, started: make(chan struct{}), closing: make(chan struct{}), release: make(chan struct{}), closed: make(chan struct{})}
	t.Cleanup(func() {
		close(input.release)
		w.Close()
		select {
		case <-input.closed:
		case <-time.After(5 * time.Second):
			t.Error("input Close did not finish")
		}
	})
	return input
}

func (r *execClosingInput) Read(p []byte) (int, error) {
	r.once.Do(func() { close(r.started) })
	return r.PipeReader.Read(p)
}

func (r *execClosingInput) Close() error {
	r.closes.Add(1)
	err := r.PipeReader.Close()
	close(r.closing)
	<-r.release
	close(r.closed)
	return err
}

type execErrorWriter struct{ err error }

func (w execErrorWriter) Write([]byte) (int, error) { return 0, w.err }

type execErrorReader struct{ err error }

func (r execErrorReader) Read([]byte) (int, error) { return 0, r.err }

type execBlockedReader struct {
	started chan struct{}
	release chan struct{}
}

func (r execBlockedReader) Read([]byte) (int, error) { close(r.started); <-r.release; return 0, io.EOF }

type execOutput struct {
	bytes.Buffer
	closes atomic.Int32
}

func (w *execOutput) Close() error { w.closes.Add(1); return nil }

func execStreamClient(t *testing.T, serve func(net.Conn)) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		if _, err := io.WriteString(conn, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n"); err != nil {
			t.Error(err)
			return
		}
		serve(conn)
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client.SkipServerVersionCheck = true
	return client
}

func execStreamPayload(raw bool) []byte {
	if raw {
		return []byte("hello")
	}
	return append([]byte{1, 0, 0, 0, 0, 0, 0, 5}, []byte("hello")...)
}

func execWaitResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("exec did not finish")
		return nil
	}
}

func TestExecOutputClosesInput(t *testing.T) {
	for _, raw := range []bool{true, false} {
		for _, outputFails := range []bool{false, true} {
			name := "multiplexed"
			if raw {
				name = "raw"
			}
			if outputFails {
				name += "/output-error"
			}
			t.Run(name, func(t *testing.T) {
				input := newExecClosingInput(t)
				outputErr := errors.New("output failed")
				client := execStreamClient(t, func(conn net.Conn) { <-input.started; conn.Write(execStreamPayload(raw)) })
				var output execOutput
				var writer io.Writer = &output
				var want error
				if outputFails {
					writer = execErrorWriter{outputErr}
					want = outputErr
				}
				result := make(chan error, 1)
				go func() {
					result <- client.StartExec("test", StartExecOptions{InputStream: input, OutputStream: writer, RawTerminal: raw})
				}()
				if err := execWaitResult(t, result); !errors.Is(err, want) {
					t.Errorf("StartExec = %v; want %v", err, want)
				}
				select {
				case <-input.closing:
				case <-time.After(5 * time.Second):
					t.Fatal("input was not closed")
				}
				if got := input.closes.Load(); got != 1 {
					t.Errorf("input Close calls = %d; want 1", got)
				}
				if !outputFails && output.String() != "hello" {
					t.Errorf("output = %q", output.String())
				}
				if output.closes.Load() != 0 {
					t.Error("client closed caller-owned output")
				}
			})
		}
	}
}

func TestExecPreservesInputError(t *testing.T) {
	for _, raw := range []bool{true, false} {
		for _, inputErr := range []error{io.ErrClosedPipe, errors.New("input failed")} {
			t.Run(map[bool]string{true: "raw", false: "multiplexed"}[raw]+"/"+inputErr.Error(), func(t *testing.T) {
				client := execStreamClient(t, func(conn net.Conn) {
					// EOF means CloseWrite ran, after the real input error was published.
					io.Copy(io.Discard, conn)
					conn.Write(execStreamPayload(raw))
				})
				result := make(chan error, 1)
				go func() {
					result <- client.StartExec("test", StartExecOptions{InputStream: execErrorReader{inputErr}, OutputStream: io.Discard, RawTerminal: raw})
				}()
				if err := execWaitResult(t, result); !errors.Is(err, inputErr) {
					t.Errorf("StartExec = %v; want %v", err, inputErr)
				}
			})
		}
	}
}

func TestExecOutputWithBlockedNonCloser(t *testing.T) {
	for _, raw := range []bool{true, false} {
		t.Run(map[bool]string{true: "raw", false: "multiplexed"}[raw], func(t *testing.T) {
			input := execBlockedReader{started: make(chan struct{}), release: make(chan struct{})}
			defer close(input.release)
			client := execStreamClient(t, func(conn net.Conn) { <-input.started; conn.Write(execStreamPayload(raw)) })
			result := make(chan error, 1)
			go func() {
				result <- client.StartExec("test", StartExecOptions{InputStream: input, OutputStream: io.Discard, RawTerminal: raw})
			}()
			if err := execWaitResult(t, result); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestExecCloseCancelsStreams(t *testing.T) {
	for _, raw := range []bool{true, false} {
		t.Run(map[bool]string{true: "raw", false: "multiplexed"}[raw], func(t *testing.T) {
			input := newExecClosingInput(t)
			releaseServer := make(chan struct{})
			defer close(releaseServer)
			client := execStreamClient(t, func(net.Conn) { <-releaseServer })
			var output execOutput
			cw, err := client.StartExecNonBlocking("test", StartExecOptions{InputStream: input, OutputStream: &output, RawTerminal: raw})
			if err != nil {
				t.Fatal(err)
			}
			select {
			case <-input.started:
			case <-time.After(5 * time.Second):
				t.Fatal("input copier did not start")
			}
			if err := cw.Close(); err != nil {
				t.Fatal(err)
			}
			result := make(chan error, 1)
			go func() { result <- cw.Wait() }()
			if err := execWaitResult(t, result); err != nil {
				t.Fatal(err)
			}
			select {
			case <-input.closing:
			case <-time.After(5 * time.Second):
				t.Fatal("input was not closed on cancellation")
			}
			if input.closes.Load() != 1 || output.closes.Load() != 0 {
				t.Fatal("unexpected stream Close ownership")
			}
		})
	}
}
