// Copyright 2026 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

const execContextTestTimeout = 2 * time.Second

func newExecContextClient(t *testing.T, raw, stallUpgrade bool) (*Client, <-chan struct{}, <-chan struct{}, func()) {
	t.Helper()
	accepted := make(chan struct{})
	ready := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	var once sync.Once
	finish := func() { once.Do(func() { close(release) }) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(accepted)
		defer close(finished)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		if stallUpgrade {
			close(ready)
			<-release
		}
		_, err = io.WriteString(conn, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n")
		if err != nil {
			return
		}
		if !stallUpgrade {
			close(ready)
			<-release
		}
		if !raw {
			conn.Write([]byte{1, 0, 0, 0, 0, 0, 0, 5})
		}
		io.WriteString(conn, "hello")
	}))
	t.Cleanup(func() {
		finish()
		server.Close()
		select {
		case <-accepted:
			waitExecContextReady(t, finished)
		default:
		}
	})
	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client.SkipServerVersionCheck = true
	return client, ready, accepted, finish
}

func waitExecContextReady(t *testing.T, ready <-chan struct{}) {
	t.Helper()
	select {
	case <-ready:
	case <-time.After(execContextTestTimeout):
		t.Fatal("exec test did not reach the expected stage")
	}
}

func startExecForContextTest(client *Client, opts StartExecOptions, nonBlocking bool) (<-chan error, <-chan struct{}) {
	result := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if !nonBlocking {
			result <- client.StartExec("test", opts)
			return
		}
		cw, err := client.StartExecNonBlocking("test", opts)
		if err != nil {
			result <- err
			return
		}
		defer cw.Close()
		result <- cw.Wait()
	}()
	return result, done
}

func waitExecContextResult(t *testing.T, result <-chan error, release func()) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(execContextTestTimeout):
		t.Error("exec did not finish after context cancellation")
	}
	// Release the server and any Success handshake so the original implementation
	// can finish too, instead of leaking its blocked goroutines when this test fails.
	release()
	select {
	case err := <-result:
		return err
	case <-time.After(execContextTestTimeout):
		t.Fatal("exec did not finish during test cleanup")
		return nil
	}
}

func TestStartExecContextCancellation(t *testing.T) {
	for _, raw := range []bool{false, true} {
		for _, nonBlocking := range []bool{false, true} {
			name := map[bool]string{true: "raw", false: "multiplexed"}[raw]
			name += map[bool]string{true: "/nonblocking", false: "/blocking"}[nonBlocking]
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				client, ready, _, finish := newExecContextClient(t, raw, false)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				result, done := startExecForContextTest(client, StartExecOptions{
					Context: ctx, OutputStream: io.Discard, RawTerminal: raw,
				}, nonBlocking)
				waitExecContextReady(t, ready)
				cancel()
				if err := waitExecContextResult(t, result, finish); !errors.Is(err, context.Canceled) {
					t.Errorf("exec returned %v; want context.Canceled", err)
				}
				waitExecContextReady(t, done)
			})
		}
	}
}

func TestStartExecContextAlreadyDone(t *testing.T) {
	for _, expired := range []bool{false, true} {
		name := map[bool]string{true: "expired", false: "canceled"}[expired]
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client, _, accepted, finish := newExecContextClient(t, true, false)
			ctx, cancel := context.WithCancel(context.Background())
			if expired {
				cancel()
				ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			}
			cancel()
			result, done := startExecForContextTest(client, StartExecOptions{
				Context: ctx, OutputStream: io.Discard, RawTerminal: true,
			}, false)
			if err := waitExecContextResult(t, result, finish); !errors.Is(err, ctx.Err()) {
				t.Errorf("exec returned %v; want %v", err, ctx.Err())
			}
			waitExecContextReady(t, done)
			select {
			case <-accepted:
				t.Error("request sent with an already canceled context")
			default:
			}
		})
	}
}

func TestStartExecContextDuringHandshake(t *testing.T) {
	for _, stage := range []string{"upgrade", "success-send", "success-ack"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			client, ready, _, finish := newExecContextClient(t, true, stage == "upgrade")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			opts := StartExecOptions{Context: ctx, OutputStream: io.Discard, RawTerminal: true}
			if stage != "upgrade" {
				opts.Success = make(chan struct{})
			}
			result, done := startExecForContextTest(client, opts, false)
			waitExecContextReady(t, ready)
			if stage == "success-ack" {
				waitExecContextReady(t, opts.Success)
			}
			cancel()
			release := func() {
				finish()
				if opts.Success == nil {
					return
				}
				go func() {
					if stage == "success-send" {
						select {
						case <-opts.Success:
						case <-done:
							return
						}
					}
					select {
					case opts.Success <- struct{}{}:
					case <-done:
					}
				}()
			}
			if err := waitExecContextResult(t, result, release); !errors.Is(err, context.Canceled) {
				t.Errorf("exec returned %v; want context.Canceled", err)
			}
			waitExecContextReady(t, done)
		})
	}
}

func TestStartExecContextNormalCompletion(t *testing.T) {
	for _, raw := range []bool{false, true} {
		for _, nilContext := range []bool{false, true} {
			name := map[bool]string{true: "raw", false: "multiplexed"}[raw]
			name += map[bool]string{true: "/nil-context", false: "/background"}[nilContext]
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				client, ready, _, finish := newExecContextClient(t, raw, false)
				var ctx context.Context
				if !nilContext {
					ctx = context.Background()
				}
				var output bytes.Buffer
				result, done := startExecForContextTest(client, StartExecOptions{
					Context: ctx, OutputStream: &output, RawTerminal: raw,
				}, true)
				waitExecContextReady(t, ready)
				finish()
				if err := waitExecContextResult(t, result, finish); err != nil {
					t.Fatal(err)
				}
				waitExecContextReady(t, done)
				if output.String() != "hello" {
					t.Errorf("output = %q; want hello", output.String())
				}
			})
		}
	}
}

type execContextBlockingWriter struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (w *execContextBlockingWriter) Write(p []byte) (int, error) {
	w.once.Do(func() {
		close(w.started)
		<-w.release
	})
	return len(p), nil
}

func TestStartExecContextWaitsForOutput(t *testing.T) {
	for _, raw := range []bool{false, true} {
		t.Run(map[bool]string{true: "raw", false: "multiplexed"}[raw], func(t *testing.T) {
			t.Parallel()
			client, ready, _, finish := newExecContextClient(t, raw, false)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			output := &execContextBlockingWriter{started: make(chan struct{}), release: make(chan struct{})}
			var releaseOnce sync.Once
			releaseOutput := func() { releaseOnce.Do(func() { close(output.release) }) }
			defer releaseOutput()
			result, done := startExecForContextTest(client, StartExecOptions{
				Context: ctx, OutputStream: output, RawTerminal: raw,
			}, true)
			waitExecContextReady(t, ready)
			finish()
			waitExecContextReady(t, output.started)
			cancel()

			var err error
			returned := false
			select {
			case err = <-result:
				returned = true
				t.Error("Wait returned while the output writer was still active")
			case <-time.After(100 * time.Millisecond):
			}
			releaseOutput()
			if !returned {
				err = waitExecContextResult(t, result, finish)
			}
			if !errors.Is(err, context.Canceled) {
				t.Errorf("exec returned %v; want context.Canceled", err)
			}
			waitExecContextReady(t, done)
		})
	}
}

type execContextReader struct {
	read func([]byte) (int, error)
}

func (r execContextReader) Read(p []byte) (int, error) { return r.read(p) }

func TestStartExecContextBlockedInput(t *testing.T) {
	for _, cancelContext := range []bool{false, true} {
		t.Run(map[bool]string{true: "context", false: "close"}[cancelContext], func(t *testing.T) {
			t.Parallel()
			client, ready, _, finish := newExecContextClient(t, true, false)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			input, inputWriter := io.Pipe()
			defer input.Close()
			defer inputWriter.Close()
			inputStarted := make(chan struct{})
			inputFinished := make(chan struct{})
			// Expose only Read so the client cannot close the blocked input.
			reader := execContextReader{read: func(p []byte) (int, error) {
				close(inputStarted)
				defer close(inputFinished)
				return input.Read(p)
			}}
			cw, err := client.StartExecNonBlocking("test", StartExecOptions{
				Context: ctx, InputStream: reader, OutputStream: io.Discard, RawTerminal: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			result := make(chan error, 1)
			go func() { result <- cw.Wait() }()
			waitExecContextReady(t, ready)
			waitExecContextReady(t, inputStarted)
			var want error
			if cancelContext {
				cancel()
				defer cw.Close()
				want = context.Canceled
			} else if err := cw.Close(); err != nil {
				t.Fatal(err)
			}
			if err := waitExecContextResult(t, result, finish); !errors.Is(err, want) {
				t.Errorf("exec returned %v; want %v", err, want)
			}
			input.Close()
			waitExecContextReady(t, inputFinished)
		})
	}
}
