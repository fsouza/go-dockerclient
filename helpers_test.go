// Copyright 2020 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"errors"
	"testing"
)

func expectNoSuchContainer(t *testing.T, id string, err error) {
	t.Helper()
	var containerErr *NoSuchContainer
	if !errors.As(err, &containerErr) {
		t.Fatalf("Container: Wrong error information. Want %#v. Got %#v.", containerErr, err)
	}
	if containerErr.ID != id {
		t.Errorf("Container: wrong container in error\nWant %q\ngot  %q", id, containerErr.ID)
	}
}
