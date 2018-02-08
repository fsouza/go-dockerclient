package docker

import (
	"context"
	"encoding/json"
	"net/http"

	"reflect"

	"testing"
)

func TestListPlugins(t *testing.T) {
	t.Parallel()
	jsonPlugins := `[
  {
    "Id": "5724e2c8652da337ab2eedd19fc6fc0ec908e4bd907c7421bf6a8dfc70c4c078",
    "Name": "tiborvass/sample-volume-plugin",
    "Tag": "latest",
    "Active": true,
    "Settings": {
      "Env": [
        "DEBUG=0"
      ],
      "Args": null,
      "Devices": null
    },
    "Config": {
      "Description": "A sample volume plugin for Docker",
      "Documentation": "https://docs.docker.com/engine/extend/plugins/",
      "Interface": {
        "Types": [
          "docker.volumedriver/1.0"
        ],
        "Socket": "plugins.sock"
      },
      "Entrypoint": [
        "/usr/bin/sample-volume-plugin",
        "/data"
      ],
      "WorkDir": "",
      "User": {},
      "Network": {
        "Type": ""
      },
      "Linux": {
        "Capabilities": null,
        "AllowAllDevices": false,
        "Devices": null
      },
      "Mounts": null,
      "PropagatedMount": "/data",
      "Env": [
        {
          "Name": "DEBUG",
          "Description": "If set, prints debug messages",
          "Settable": null,
          "Value": "0"
        }
      ],
      "Args": {
        "Name": "args",
        "Description": "command line arguments",
        "Settable": null,
        "Value": []
      }
    }
  }
]`
	var expected []PluginDetail
	err := json.Unmarshal([]byte(jsonPlugins), &expected)
	if err != nil {
		t.Fatal(err)
	}
	client := newTestClient(&FakeRoundTripper{message: jsonPlugins, status: http.StatusOK})
	pluginDetails, err := client.ListPlugins()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pluginDetails, expected) {
		t.Errorf("ListPlugins: Expected %#v. Got %#v.", expected, pluginDetails)
	}
}

func TestGetPluginPrivileges(t *testing.T) {
	t.Parallel()
	name := "test_plugin"
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusNoContent}
	client := newTestClient(fakeRT)
	var expected []PluginPrivilege
	pluginPrivileges, err := client.GetPluginPrivileges(name)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pluginPrivileges, expected) {
		t.Errorf("PluginPrivileges: Expected %#v. Got %#v.", expected, pluginPrivileges)
	}
}

func TestInstallPlugins(t *testing.T) {
	opts := InstallPluginOptions{
		Remote: "", Name: "test",
		Plugins: []PluginPrivilege{
			{
				Name:        "network",
				Description: "",
				Value:       []string{"host"},
			},
		},
		Context: context.Background(),
	}
	auth := AuthConfiguration{}
	client := newTestClient(&FakeRoundTripper{message: "", status: http.StatusOK})
	err := client.InstallPlugins(opts, auth)
	if err != nil {
		t.Fatal(err)
	}

}
