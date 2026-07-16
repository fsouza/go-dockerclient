// Copyright 2026 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// defaultContextName is the reserved name docker CLI uses for the
	// implicit context (DOCKER_HOST / default socket).
	defaultContextName = "default"

	// contextDockerEndpoint is the key under Endpoints in a context's
	// meta.json that holds the docker engine endpoint.
	contextDockerEndpoint = "docker"
)

// contextMeta mirrors the JSON written by docker CLI in
// $DOCKER_CONFIG/contexts/meta/<id>/meta.json.
type contextMeta struct {
	Name      string                         `json:"Name"`
	Metadata  map[string]any                 `json:"Metadata"`
	Endpoints map[string]contextMetaEndpoint `json:"Endpoints"`
}

type contextMetaEndpoint struct {
	Host          string `json:"Host"`
	SkipTLSVerify bool   `json:"SkipTLSVerify"`
}

type dockerConfigJSON struct {
	CurrentContext string `json:"currentContext"`
}

type contextTLSData struct {
	CA   []byte
	Cert []byte
	Key  []byte
}

// resolvedContext is the resolved view of a docker CLI context: a host plus
// optional TLS material loaded from $DOCKER_CONFIG/contexts/tls/<id>/docker.
type resolvedContext struct {
	Name    string
	Host    string
	TLSData *contextTLSData
}

// dockerConfigDir returns the docker CLI config directory, honoring
// DOCKER_CONFIG and falling back to ~/.docker.
func dockerConfigDir() (string, error) {
	if dir := os.Getenv("DOCKER_CONFIG"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if home == "" {
		return "", errors.New("could not determine docker config dir: HOME is not set")
	}
	return filepath.Join(home, ".docker"), nil
}

// currentContextName returns the context name docker CLI would use when
// DOCKER_HOST is unset. It honors DOCKER_CONTEXT first, then the
// currentContext field of config.json, and falls back to "default".
func currentContextName() (string, error) {
	if name := os.Getenv("DOCKER_CONTEXT"); name != "" {
		return name, nil
	}
	dir, err := dockerConfigDir()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return defaultContextName, nil
		}
		return "", err
	}
	var cfg dockerConfigJSON
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("invalid docker config file: %w", err)
	}
	if cfg.CurrentContext == "" {
		return defaultContextName, nil
	}
	return cfg.CurrentContext, nil
}

// contextDirID returns the directory name docker CLI uses to store a
// context's metadata and TLS data: the hex-encoded sha256 of the name.
func contextDirID(name string) string {
	sum := sha256.Sum256([]byte(name))
	return hex.EncodeToString(sum[:])
}

// loadContext loads the docker CLI context with the given name. The reserved
// name "default" (and the empty string) resolve to the platform default host
// without consulting any files on disk.
func loadContext(name string) (*resolvedContext, error) {
	if name == "" || name == defaultContextName {
		return &resolvedContext{Name: defaultContextName, Host: defaultHost}, nil
	}
	dir, err := dockerConfigDir()
	if err != nil {
		return nil, err
	}
	id := contextDirID(name)
	metaPath := filepath.Join(dir, "contexts", "meta", id, "meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("docker context %q not found", name)
		}
		return nil, err
	}
	var meta contextMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("invalid metadata for docker context %q: %w", name, err)
	}
	ep, ok := meta.Endpoints[contextDockerEndpoint]
	if !ok {
		return nil, fmt.Errorf("docker context %q has no %q endpoint", name, contextDockerEndpoint)
	}
	if ep.Host == "" {
		return nil, fmt.Errorf("docker context %q has an empty host", name)
	}
	if strings.HasPrefix(ep.Host, "ssh://") {
		return nil, fmt.Errorf("docker context %q uses an ssh:// endpoint, which is not supported by go-dockerclient", name)
	}
	tlsDir := filepath.Join(dir, "contexts", "tls", id, contextDockerEndpoint)
	tlsData, err := loadContextTLSData(tlsDir)
	if err != nil {
		return nil, fmt.Errorf("loading TLS data for docker context %q: %w", name, err)
	}
	return &resolvedContext{Name: name, Host: ep.Host, TLSData: tlsData}, nil
}

// loadContextTLSData reads ca.pem, cert.pem, and key.pem from dir. Missing
// files are silently skipped — docker CLI itself allows partial TLS data, and
// returning nil here means "no TLS for this context".
func loadContextTLSData(dir string) (*contextTLSData, error) {
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := &contextTLSData{}
	entries := []struct {
		name string
		dst  *[]byte
	}{
		{"ca.pem", &out.CA},
		{"cert.pem", &out.Cert},
		{"key.pem", &out.Key},
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.name))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		*e.dst = b
	}
	if out.CA == nil && out.Cert == nil && out.Key == nil {
		return nil, nil
	}
	return out, nil
}

// NewContextClient returns a Client built from the named docker CLI context.
// Pass "default" (or "") to use the platform default host. The named context
// must already exist in $DOCKER_CONFIG/contexts (created via `docker context
// create`).
func NewContextClient(name string) (*Client, error) {
	return NewVersionedContextClient(name, "")
}

// NewVersionedContextClient is like NewContextClient but pins the client to a
// specific remote API version.
func NewVersionedContextClient(name, apiVersionString string) (*Client, error) {
	ctx, err := loadContext(name)
	if err != nil {
		return nil, err
	}
	return clientFromResolvedContext(ctx, apiVersionString)
}

func clientFromResolvedContext(ctx *resolvedContext, apiVersionString string) (*Client, error) {
	var (
		client *Client
		err    error
	)
	if ctx.TLSData != nil {
		client, err = NewVersionedTLSClientFromBytes(ctx.Host, ctx.TLSData.Cert, ctx.TLSData.Key, ctx.TLSData.CA, apiVersionString)
	} else {
		client, err = NewVersionedClient(ctx.Host, apiVersionString)
	}
	if err != nil {
		return nil, err
	}
	client.SkipServerVersionCheck = apiVersionString == ""
	return client, nil
}
