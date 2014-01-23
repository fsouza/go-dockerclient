package engine

import (
	"io/ioutil"
	"testing"
)

var globalTestID string

func newTestEngine(t *testing.T) *Engine {
	tmp, err := ioutil.TempDir("", "asd")
	if err != nil {
		t.Fatal(err)
	}
	eng, err := New(tmp)
	if err != nil {
		t.Fatal(err)
	}
	return eng
}

func mkJob(t *testing.T, name string, args ...string) *Job {
	return newTestEngine(t).Job(name, args...)
}
