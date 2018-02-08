// Copyright 2013 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"encoding/json"
	ioutil "io/ioutil"
	"net/http"
	"net/url"

	"golang.org/x/net/context"
)

// PluginPrivilege represents a privilege for a plugin.
type PluginPrivilege struct {
	Name        string   `json:"Name,omitempty" yaml:"Name,omitempty" toml:"Name,omitempty"`
	Description string   `json:"Description,omitempty" yaml:"Description,omitempty" toml:"Description,omitempty"`
	Value       []string `json:"Value,omitempty" yaml:"Value,omitempty" toml:"Value,omitempty"`
}

// InstallPluginOptions .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
type InstallPluginOptions struct {
	Remote  string
	Name    string
	Plugins []PluginPrivilege
	Context context.Context
}

// InstallPlugins returns a slice of containers matching the given criteria.
//
// See https://goo.gl/kaOHGw for more details.
func (c *Client) InstallPlugins(opts InstallPluginOptions, auth AuthConfiguration) error {
	params := make(url.Values)
	params.Set("remote", opts.Remote)
	if opts.Name != "" {
		params.Set("name", opts.Name)
	}
	path := "/plugins/pull?" + queryString(params)
	resp, err := c.do("POST", path, doOptions{
		data:    opts.Plugins,
		context: opts.Context,
	})
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	return nil
}

// PluginSetting .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
type PluginSetting struct {
	Env     []string `json:"Env,omitempty" yaml:"Env,omitempty" toml:"Env,omitempty"`
	Args    []string `json:"Args,omitempty" yaml:"Args,omitempty" toml:"Args,omitempty"`
	Devices []string `json:"Devices,omitempty" yaml:"Devices,omitempty" toml:"Devices,omitempty"`
}

// PluginInterface .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
type PluginInterface struct {
	Types  []string `json:"Types,omitempty" yaml:"Types,omitempty" toml:"Types,omitempty"`
	Socket string   `json:"Socket,omitempty" yaml:"Socket,omitempty" toml:"Socket,omitempty"`
}

// PluginNetwork .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
type PluginNetwork struct {
	Type string `json:"Type,omitempty" yaml:"Type,omitempty" toml:"Type,omitempty"`
}

// PluginLinux .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
type PluginLinux struct {
	Capabilities    []string             `json:"Capabilities,omitempty" yaml:"Capabilities,omitempty" toml:"Capabilities,omitempty"`
	AllowAllDevices bool                 `json:"AllowAllDevices,omitempty" yaml:"AllowAllDevices,omitempty" toml:"AllowAllDevices,omitempty"`
	Devices         []PluginLinuxDevices `json:"Devices,omitempty" yaml:"Devices,omitempty" toml:"Devices,omitempty"`
}

// PluginLinuxDevices .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
type PluginLinuxDevices struct {
	Name        string   `json:"Name,omitempty" yaml:"Name,omitempty" toml:"Name,omitempty"`
	Description string   `json:"Documentation,omitempty" yaml:"Documentation,omitempty" toml:"Documentation,omitempty"`
	Settable    []string `json:"Settable,omitempty" yaml:"Settable,omitempty" toml:"Settable,omitempty"`
	Path        string   `json:"Path,omitempty" yaml:"Path,omitempty" toml:"Path,omitempty"`
}

// PluginEnv .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
type PluginEnv struct {
	Name        string   `json:"Name,omitempty" yaml:"Name,omitempty" toml:"Name,omitempty"`
	Description string   `json:"Description,omitempty" yaml:"Description,omitempty" toml:"Description,omitempty"`
	Settable    []string `json:"Settable,omitempty" yaml:"Settable,omitempty" toml:"Settable,omitempty"`
	Value       string   `json:"Value,omitempty" yaml:"Value,omitempty" toml:"Value,omitempty"`
}

// PluginArgs .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
type PluginArgs struct {
	Name        string   `json:"Name,omitempty" yaml:"Name,omitempty" toml:"Name,omitempty"`
	Description string   `json:"Description,omitempty" yaml:"Description,omitempty" toml:"Description,omitempty"`
	Settable    []string `json:"Settable,omitempty" yaml:"Settable,omitempty" toml:"Settable,omitempty"`
	Value       []string `json:"Value,omitempty" yaml:"Value,omitempty" toml:"Value,omitempty"`
}

// PluginUser .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
type PluginUser struct {
	UID int32 `json:"UID,omitempty" yaml:"UID,omitempty" toml:"UID,omitempty"`
	GID int32 `json:"GID,omitempty" yaml:"GID,omitempty" toml:"GID,omitempty"`
}

// PluginConfig .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
type PluginConfig struct {
	Description     string `json:"Description,omitempty" yaml:"Description,omitempty" toml:"Description,omitempty"`
	Documentation   string
	Interface       PluginInterface `json:"Interface,omitempty" yaml:"Interface,omitempty" toml:"Interface,omitempty"`
	Entrypoint      []string        `json:"Entrypoint,omitempty" yaml:"Entrypoint,omitempty" toml:"Entrypoint,omitempty"`
	WorkDir         string          `json:"WorkDir,omitempty" yaml:"WorkDir,omitempty" toml:"WorkDir,omitempty"`
	User            PluginUser      `json:"User,omitempty" yaml:"User,omitempty" toml:"User,omitempty"`
	Network         PluginNetwork   `json:"Network,omitempty" yaml:"Network,omitempty" toml:"Network,omitempty"`
	Linux           PluginLinux     `json:"Linux,omitempty" yaml:"Linux,omitempty" toml:"Linux,omitempty"`
	PropagatedMount string          `json:"PropagatedMount,omitempty" yaml:"PropagatedMount,omitempty" toml:"PropagatedMount,omitempty"`
	Mounts          []Mount         `json:"Mounts,omitempty" yaml:"Mounts,omitempty" toml:"Mounts,omitempty"`
	Env             []PluginEnv     `json:"Env,omitempty" yaml:"Env,omitempty" toml:"Env,omitempty"`
	Args            PluginArgs      `json:"Args,omitempty" yaml:"Args,omitempty" toml:"Args,omitempty"`
}

// PluginDetail .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
type PluginDetail struct {
	ID       string        `json:"Id,omitempty" yaml:"Id,omitempty" toml:"Id,omitempty"`
	Name     string        `json:"Name,omitempty" yaml:"Name,omitempty" toml:"Name,omitempty"`
	Tag      string        `json:"Tag,omitempty" yaml:"Tag,omitempty" toml:"Tag,omitempty"`
	Active   bool          `json:"Active,omitempty" yaml:"Active,omitempty" toml:"Active,omitempty"`
	Settings PluginSetting `json:"Settings,omitempty" yaml:"Settings,omitempty" toml:"Settings,omitempty"`
	Config   PluginConfig  `json:"Config,omitempty" yaml:"Config,omitempty" toml:"Config,omitempty"`
}

// ListPlugins .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
func (c *Client) ListPlugins() ([]PluginDetail, error) {
	resp, err := c.do("GET", "/plugins", doOptions{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	pluginDetails := make([]PluginDetail, 0)
	if err := json.NewDecoder(resp.Body).Decode(&pluginDetails); err != nil {
		return nil, err
	}
	return pluginDetails, nil
}

// GetPluginPrivileges .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
func (c *Client) GetPluginPrivileges(name string) ([]PluginPrivilege, error) {
	resp, err := c.do("GET", "/plugins/privileges?"+name, doOptions{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var pluginPrivileges []PluginPrivilege
	if err := json.NewDecoder(resp.Body).Decode(&pluginPrivileges); err != nil {
		return nil, err
	}
	return pluginPrivileges, nil
}

// InspectPlugins .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
func (c *Client) InspectPlugins(name string) (*PluginDetail, error) {
	resp, err := c.do("GET", "/plugins/"+name+"/json", doOptions{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err != nil {
		if e, ok := err.(*Error); ok && e.Status == http.StatusNotFound {
			return nil, &NoSuchPlugin{ID: name}
		}
		return nil, err
	}
	resp.Body.Close()
	var pluginDetail PluginDetail
	if err := json.NewDecoder(resp.Body).Decode(&pluginDetail); err != nil {
		return nil, err
	}
	return &pluginDetail, nil
}

// RemovePluginOptions .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
type RemovePluginOptions struct {
	// The ID of the container.
	Name string `qs:"-"`

	// A flag that indicates whether Docker should remove the plugin
	// even if it is currently used.
	Force   bool `qs:"force"`
	Context context.Context
}

// RemovePlugin .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
func (c *Client) RemovePlugin(opts RemovePluginOptions) (*PluginDetail, error) {
	path := "/plugins/" + opts.Name + "?" + queryString(opts)
	resp, err := c.do("DELETE", path, doOptions{context: opts.Context})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err != nil {
		if e, ok := err.(*Error); ok && e.Status == http.StatusNotFound {
			return nil, &NoSuchPlugin{ID: opts.Name}
		}
		return nil, err
	}
	resp.Body.Close()
	var pluginDetail PluginDetail
	if err := json.NewDecoder(resp.Body).Decode(&pluginDetail); err != nil {
		return nil, err
	}
	return &pluginDetail, nil
}

// EnablePluginOptions .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
type EnablePluginOptions struct {
	// The ID of the container.
	Name    string `qs:"-"`
	Timeout int64  `qs:"timeout"`

	Context context.Context
}

// EnablePlugin .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
func (c *Client) EnablePlugin(opts EnablePluginOptions) error {
	path := "/plugins/" + opts.Name + "/enable?" + queryString(opts)
	resp, err := c.do("POST", path, doOptions{context: opts.Context})
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// DisablePluginOptions .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
type DisablePluginOptions struct {
	// The ID of the container.
	Name string `qs:"-"`

	Context context.Context
}

// DisablePlugin .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
func (c *Client) DisablePlugin(opts DisablePluginOptions) error {
	path := "/plugins/" + opts.Name + "/disable"
	resp, err := c.do("POST", path, doOptions{context: opts.Context})
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// CreatePluginOptions .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
type CreatePluginOptions struct {
	// The Name of the container.
	Name string `qs:"name"`
	// Path to tar containing plugin
	Path string `qs:"-"`

	Context context.Context
}

// CreatePlugin .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
func (c *Client) CreatePlugin(opts CreatePluginOptions) (string, error) {
	path := "/plugins/create?" + queryString(opts.Name)
	resp, err := c.do("POST", path, doOptions{
		data:    opts.Path,
		context: opts.Context})
	defer resp.Body.Close()
	if err != nil {
		return "", err
	}
	containerNameBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(containerNameBytes), nil
}

// PushPluginOptions .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
type PushPluginOptions struct {
	// The Name of the container.
	Name string

	Context context.Context
}

// PushPlugin .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
func (c *Client) PushPlugin(opts PushPluginOptions) error {
	path := "/plugins/" + opts.Name + "/push"
	resp, err := c.do("POST", path, doOptions{context: opts.Context})
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	return nil
}

// ConfigurePluginOptions .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
type ConfigurePluginOptions struct {
	// The Name of the container.
	Name string `qs:"name"`
	Envs []string

	Context context.Context
}

// ConfigurePlugin .This is a TBD Comments.
//
// See https://goo.gl/kaOHGw for more details.
func (c *Client) ConfigurePlugin(opts ConfigurePluginOptions) error {
	path := "/plugins/" + opts.Name + "/set"
	resp, err := c.do("POST", path, doOptions{
		data:    opts.Envs,
		context: opts.Context,
	})
	defer resp.Body.Close()
	if err != nil {
		if e, ok := err.(*Error); ok && e.Status == http.StatusNotFound {
			return &NoSuchPlugin{ID: opts.Name}
		}
		return err
	}
	return nil
}

// NoSuchPlugin is the error returned when a given plugin does not exist.
type NoSuchPlugin struct {
	ID  string
	Err error
}

func (err *NoSuchPlugin) Error() string {
	if err.Err != nil {
		return err.Err.Error()
	}
	return "No such plugin: " + err.ID
}
