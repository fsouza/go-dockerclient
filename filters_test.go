// Copyright 2020 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"testing"
)

func TestArgs(t *testing.T) {
	t.Parallel()
	filters := NewArgs()
	serialized := filters.ToJSON()
	if serialized != "" {
		t.Errorf("Incorrectly serialized %s", serialized)
	}
	filters.Add("status", "paused")
	filters.Add("status", "running")
	length := filters.Len()
	if length != 1 {
		t.Errorf("Incorrect length %d", length)
	}

	serialized = filters.ToJSON()
	if serialized != "{\"status\":[\"paused\",\"running\"]}" {
		t.Errorf("Incorrectly serialized %s", serialized)
	}
}
