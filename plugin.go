// Copyright 2013 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"encoding/json"
	"net/http"
	"net/url"
	"golang.org/x/net/context"
	ioutil "io/ioutil"
)


// PluginPrivilege represents a privilege for a plugin.
type PluginPrivilege struct {
	Name        string `json:"Name,omitempty" yaml:"Name,omitempty" toml:"Name,omitempty"`
	Description string `json:"Description,omitempty" yaml:"Description,omitempty" toml:"Description,omitempty"`
	Value []string `json:"Value,omitempty" yaml:"Value,omitempty" toml:"Value,omitempty"`
}

type InstallPluginOptions struct {
	Remote string
	Name string
	Plugins []PluginPrivilege
	Context context.Context
}

// ListContainers returns a slice of containers matching the given criteria.
//
// See https://goo.gl/kaOHGw for more details.
func (c *Client) InstallPlugins(opts InstallPluginOptions, auth AuthConfiguration) (error) {
	params := make(url.Values)
	params.Set("remote", opts.Remote)
	if opts.Name != ""{
		params.Set("name", opts.Name)
	}
	path := "/plugins/pull?" + queryString(params)
	resp, err := c.do("POST", path, doOptions{
		data: opts.Plugins,
		context:   opts.Context,
	})
	defer resp.Body.Close()
	if err != nil {return err}
	return  nil
}

type PluginSetting struct {
	Env []string `json:"Env,omitempty" yaml:"Env,omitempty" toml:"Env,omitempty"`
	Args []string  `json:"Args,omitempty" yaml:"Args,omitempty" toml:"Args,omitempty"`
	Devices []string  `json:"Devices,omitempty" yaml:"Devices,omitempty" toml:"Devices,omitempty"`
}

type PluginInterfaceType struct {
	Preifx string
	Capability string
	Version string
}

type PluginInterface struct {
	Types []PluginInterfaceType
	Socket string
}

type PluginNetwork struct {
	Type string
}
type PluginLinux struct {
	Capabilities []string
	AllowAllDevices bool
	Devices []PluginLinuxDevices
}

type PluginLinuxDevices struct {
	Name string
	Description string
	Settable []string
	Path string
}
type PluginEnv struct {
	Name string
	Description string
	Settable []string
	Value string
}
type PluginArgs struct {
	Name string
	Description string
	Settable []string
	Value []string
}

type PluginUser struct {
	UID int32
	GID int32
}

type PluginConfig struct {
	Description string `json:"Description,omitempty" yaml:"Description,omitempty" toml:"Description,omitempty"`
	Documentation string `json:"Documentation,omitempty" yaml:"Documentation,omitempty" toml:"Documentation,omitempty"`
	Interface PluginInterface
	Entrypoint []string
	WorkDir string
	User PluginUser
	Network PluginNetwork
	Linux PluginLinux
	PropagatedMount string
	Mounts []Mount
	Env []PluginEnv
	Args PluginArgs
}

type PluginDetail struct {
	Id string `json:"Id,omitempty" yaml:"Id,omitempty" toml:"Id,omitempty"`
	Name        string `json:"Name,omitempty" yaml:"Name,omitempty" toml:"Name,omitempty"`
	Tag        string `json:"Tag,omitempty" yaml:"Tag,omitempty" toml:"Tag,omitempty"`
	Active     bool `json:"Active,omitempty" yaml:"Active,omitempty" toml:"Active,omitempty"`
	Settings PluginSetting  `json:"Settings,omitempty" yaml:"Settings,omitempty" toml:"Settings,omitempty"`
	Config Config  `json:"Config,omitempty" yaml:"Config,omitempty" toml:"Config,omitempty"`
}

func (c *Client) ListPlugins()([]PluginDetail, error) {
	resp, err := c.do("GET", "/plugins",doOptions{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	pluginDetails := make([]PluginDetail,0)
	if err := json.NewDecoder(resp.Body).Decode(&pluginDetails); err != nil {
		return nil, err
	}
	return pluginDetails, nil
}


func (c *Client) GetPluginPrivileges(name string)([]PluginPrivilege, error) {
	resp, err := c.do("GET", "/plugins/privileges?"+name,doOptions{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	pluginPrivileges := make([]PluginPrivilege,0)
	if err := json.NewDecoder(resp.Body).Decode(&pluginPrivileges); err != nil {
		return nil, err
	}
	return pluginPrivileges, nil
}

type RemovePluginOptions struct {
	// The ID of the container.
	Name string `qs:"-"`

	// A flag that indicates whether Docker should remove the plugin
	// even if it is currently used.
	Force   bool  `qs:"force"`
	Context context.Context
}

func (c *Client) RemovePlugin(opts RemovePluginOptions)(*PluginDetail, error) {
	path := "/plugins/"+opts.Name+"?"+queryString(opts)
	resp, err := c.do("DELETE", path, doOptions{context: opts.Context})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err != nil {
		if e, ok := err.(*Error); ok && e.Status == http.StatusNotFound {
			return nil,&NoSuchPlugin{ID: opts.Name}
		}
		return nil,err
	}
	resp.Body.Close()
	var pluginDetail PluginDetail
	if err := json.NewDecoder(resp.Body).Decode(&pluginDetail); err != nil {
		return nil, err
	}
	return &pluginDetail, nil
}


type EnablePluginOptions struct {
	// The ID of the container.
	Name string `qs:"-"`
	Timeout int64 `qs:"timeout"`

	Context context.Context
}

func (c *Client) EnablePlugin(opts EnablePluginOptions)(error) {
	path := "/plugins/"+opts.Name+"/enable?"+queryString(opts)
	resp, err := c.do("POST", path, doOptions{context: opts.Context})
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

type DisablePluginOptions struct {
	// The ID of the container.
	Name string `qs:"-"`

	Context context.Context
}

func (c *Client) DisablePlugin(opts DisablePluginOptions)(error) {
	path := "/plugins/"+opts.Name+"/disable"
	resp, err := c.do("POST", path, doOptions{context: opts.Context})
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

type CreatePluginOptions struct {
	// The Name of the container.
	Name string `qs:"name"`
	// Path to tar containing plugin
	Path string `qs:"-"`

	Context context.Context
}

func (c *Client) CreatePlugin(opts CreatePluginOptions)(string,error) {
	path := "/plugins/create?"+queryString(opts.Name)
	resp, err := c.do("POST", path, doOptions{
		data:    opts.Path,
		context: opts.Context})
	defer resp.Body.Close()
	if err != nil {
		return "",err
	}
	containerNameBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "",err
	}
	return string(containerNameBytes),nil
}

type PushPluginOptions struct {
	// The Name of the container.
	Name string

	Context context.Context
}

func (c *Client) PushPlugin(opts PushPluginOptions)(error) {
	path := "/plugins/"+opts.Name+"/push"
	resp, err := c.do("POST", path, doOptions{context: opts.Context})
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	return nil
}

type ConfigurePluginOptions struct {
	// The Name of the container.
	Name string `qs:"name"`
	Envs []string

	Context context.Context
}

func (c *Client) ConfigurePlugin(opts ConfigurePluginOptions)(error) {
	path := "/plugins/"+opts.Name+"/set"
	resp, err := c.do("POST", path, doOptions{
		data:opts.Envs,
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
