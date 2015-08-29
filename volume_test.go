// Copyright 2015 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
)

func TestListVolumes(t *testing.T) {
	volumesData := `[
	{
		"Name": "tardis",
		"Driver": "local",
		"Mountpoint": "/var/lib/docker/volumes/tardis"
	},
	{
		"Name": "foo",
		"Driver": "bar",
		"Mountpoint": "/var/lib/docker/volumes/bar"
	}
]`
	body := `{ "Volumes":` + volumesData + ` }`
	var expected []APIVolumes
	err := json.Unmarshal([]byte(volumesData), &expected)
	if err != nil {
		t.Fatal(err)
	}
	client := newTestClient(&FakeRoundTripper{message: body, status: http.StatusOK})
	images, err := client.ListVolumes(ListVolumesOptions{})
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(images, expected) {
		t.Errorf("ListVolumes: Wrong return value. Want %#v. Got %#v.", expected, images)
	}
}
