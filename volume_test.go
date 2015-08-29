// Copyright 2015 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"encoding/json"
	"net/http"
	"net/url"
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
	body := `{ "Volumes": ` + volumesData + ` }`
	var expected []Volume
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

func TestRemoveVolume(t *testing.T) {
	name := "test"
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusNoContent}
	client := newTestClient(fakeRT)
	err := client.RemoveVolume(name)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	expectedMethod := "DELETE"
	if req.Method != expectedMethod {
		t.Errorf("RemoveVolume(%q): Wrong HTTP method. Want %s. Got %s.", name, expectedMethod, req.Method)
	}
	u, _ := url.Parse(client.getURL("/volumes/" + name))
	if req.URL.Path != u.Path {
		t.Errorf("RemoveVolume(%q): Wrong request path. Want %q. Got %q.", name, u.Path, req.URL.Path)
	}
}

func TestRemoveVolumeNotFound(t *testing.T) {
	client := newTestClient(&FakeRoundTripper{message: "no such volume", status: http.StatusNotFound})
	err := client.RemoveVolume("test:")
	if err != ErrNoSuchVolume {
		t.Errorf("RemoveVolume: wrong error. Want %#v. Got %#v.", ErrNoSuchVolume, err)
	}
}

func TestRemoveVolumeInUse(t *testing.T) {
	client := newTestClient(&FakeRoundTripper{message: "volume in use and cannot be removed", status: http.StatusConflict})
	err := client.RemoveVolume("test:")
	if err != ErrVolumeInUse {
		t.Errorf("RemoveVolume: wrong error. Want %#v. Got %#v.", ErrVolumeInUse, err)
	}
}
