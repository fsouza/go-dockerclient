// Copyright 2026 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeContext writes a docker CLI context with the given name and endpoint
// host into dir, plus optional TLS files. It mirrors the on-disk layout
// produced by `docker context create`.
func writeContext(t *testing.T, dir, name, host string, tls *contextTLSData) {
	t.Helper()
	id := contextDirID(name)
	metaDir := filepath.Join(dir, "contexts", "meta", id)
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := contextMeta{
		Name: name,
		Endpoints: map[string]contextMetaEndpoint{
			contextDockerEndpoint: {Host: host},
		},
	}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metaDir, "meta.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if tls == nil {
		return
	}
	tlsDir := filepath.Join(dir, "contexts", "tls", id, contextDockerEndpoint)
	if err := os.MkdirAll(tlsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if tls.CA != nil {
		if err := os.WriteFile(filepath.Join(tlsDir, "ca.pem"), tls.CA, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if tls.Cert != nil {
		if err := os.WriteFile(filepath.Join(tlsDir, "cert.pem"), tls.Cert, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if tls.Key != nil {
		if err := os.WriteFile(filepath.Join(tlsDir, "key.pem"), tls.Key, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// writeConfigCurrentContext writes a docker config.json that selects name as
// the current context.
func writeConfigCurrentContext(t *testing.T, dir, name string) {
	t.Helper()
	data, err := json.Marshal(dockerConfigJSON{CurrentContext: name})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestContextDirIDIsSha256(t *testing.T) {
	t.Parallel()
	// Cross-check against the docker CLI's published value for "default".
	// echo -n default | sha256sum
	got := contextDirID("default")
	want := "37a8eec1ce19687d132fe29051dca629d164e2c4958ba141d5f4133a33f0688f"
	if got != want {
		t.Errorf("contextDirID(%q) = %q, want %q", "default", got, want)
	}
}

func TestLoadContext_Default(t *testing.T) {
	t.Parallel()
	ctx, err := loadContext("default")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Host != defaultHost {
		t.Errorf("default context host = %q, want %q", ctx.Host, defaultHost)
	}
	if ctx.TLSData != nil {
		t.Errorf("default context should have no TLS data, got %+v", ctx.TLSData)
	}
}

func TestLoadContext_FromDisk(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	writeContext(t, dir, "remote", "tcp://example.com:2375", nil)

	ctx, err := loadContext("remote")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Host != "tcp://example.com:2375" {
		t.Errorf("host = %q, want %q", ctx.Host, "tcp://example.com:2375")
	}
	if ctx.TLSData != nil {
		t.Errorf("expected no TLS data, got %+v", ctx.TLSData)
	}
}

func TestLoadContext_WithTLSData(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	ca := []byte("-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----\n")
	cert := []byte("-----BEGIN CERTIFICATE-----\ncert\n-----END CERTIFICATE-----\n")
	key := []byte("-----BEGIN RSA PRIVATE KEY-----\nkey\n-----END RSA PRIVATE KEY-----\n")
	writeContext(t, dir, "secure", "tcp://secure.example.com:2376", &contextTLSData{
		CA: ca, Cert: cert, Key: key,
	})

	ctx, err := loadContext("secure")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.TLSData == nil {
		t.Fatal("expected TLS data, got nil")
	}
	if string(ctx.TLSData.CA) != string(ca) {
		t.Error("ca mismatch")
	}
	if string(ctx.TLSData.Cert) != string(cert) {
		t.Error("cert mismatch")
	}
	if string(ctx.TLSData.Key) != string(key) {
		t.Error("key mismatch")
	}
}

func TestLoadContext_NotFound(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	_, err := loadContext("missing")
	if err == nil {
		t.Fatal("expected error for missing context, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got %q", err)
	}
}

func TestLoadContext_SSHIsRejected(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	writeContext(t, dir, "ssh-ctx", "ssh://user@host", nil)
	_, err := loadContext("ssh-ctx")
	if err == nil {
		t.Fatal("expected error for ssh:// context, got nil")
	}
	if !strings.Contains(err.Error(), "ssh://") {
		t.Errorf("error should mention ssh://, got %q", err)
	}
}

func TestCurrentContextName_DOCKER_CONTEXT_wins(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	writeConfigCurrentContext(t, dir, "from-config")
	t.Setenv("DOCKER_CONTEXT", "from-env")

	name, err := currentContextName()
	if err != nil {
		t.Fatal(err)
	}
	if name != "from-env" {
		t.Errorf("got %q, want %q", name, "from-env")
	}
}

func TestCurrentContextName_FromConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	t.Setenv("DOCKER_CONTEXT", "")
	writeConfigCurrentContext(t, dir, "from-config")

	name, err := currentContextName()
	if err != nil {
		t.Fatal(err)
	}
	if name != "from-config" {
		t.Errorf("got %q, want %q", name, "from-config")
	}
}

func TestCurrentContextName_NoConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	t.Setenv("DOCKER_CONTEXT", "")

	name, err := currentContextName()
	if err != nil {
		t.Fatal(err)
	}
	if name != defaultContextName {
		t.Errorf("got %q, want %q", name, defaultContextName)
	}
}

func TestNewContextClient(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	writeContext(t, dir, "remote", "tcp://example.com:2375", nil)

	client, err := NewContextClient("remote")
	if err != nil {
		t.Fatal(err)
	}
	if client.endpoint != "tcp://example.com:2375" {
		t.Errorf("endpoint = %q, want %q", client.endpoint, "tcp://example.com:2375")
	}
	if !client.SkipServerVersionCheck {
		t.Error("expected SkipServerVersionCheck to be true when no API version is requested")
	}
}

func TestNewContextClient_Default(t *testing.T) {
	t.Parallel()
	client, err := NewContextClient("default")
	if err != nil {
		t.Fatal(err)
	}
	if client.endpoint != defaultHost {
		t.Errorf("endpoint = %q, want %q", client.endpoint, defaultHost)
	}
}

func TestNewVersionedContextClient(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	writeContext(t, dir, "remote", "tcp://example.com:2375", nil)

	client, err := NewVersionedContextClient("remote", "1.40")
	if err != nil {
		t.Fatal(err)
	}
	if client.endpoint != "tcp://example.com:2375" {
		t.Errorf("endpoint = %q, want %q", client.endpoint, "tcp://example.com:2375")
	}
	if got := client.requestedAPIVersion.String(); got != "1.40" {
		t.Errorf("requestedAPIVersion = %q, want %q", got, "1.40")
	}
	if client.SkipServerVersionCheck {
		t.Error("expected SkipServerVersionCheck to be false when an API version is requested")
	}
}

func TestNewClientFromEnv_UsesCurrentContext(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("DOCKER_CONTEXT", "")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	writeContext(t, dir, "remote", "tcp://example.com:2375", nil)
	writeConfigCurrentContext(t, dir, "remote")

	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if client.endpoint != "tcp://example.com:2375" {
		t.Errorf("endpoint = %q, want %q", client.endpoint, "tcp://example.com:2375")
	}
}

func TestNewClientFromEnv_DOCKER_CONTEXT_env(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	writeContext(t, dir, "via-env", "tcp://env.example.com:2375", nil)
	t.Setenv("DOCKER_CONTEXT", "via-env")

	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if client.endpoint != "tcp://env.example.com:2375" {
		t.Errorf("endpoint = %q, want %q", client.endpoint, "tcp://env.example.com:2375")
	}
}

func TestNewClientFromEnv_DOCKER_HOST_overrides_context(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	t.Setenv("DOCKER_TLS_VERIFY", "")
	writeContext(t, dir, "remote", "tcp://context.example.com:2375", nil)
	writeConfigCurrentContext(t, dir, "remote")
	t.Setenv("DOCKER_HOST", "tcp://override.example.com:2375")

	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if client.endpoint != "tcp://override.example.com:2375" {
		t.Errorf("endpoint = %q, want %q", client.endpoint, "tcp://override.example.com:2375")
	}
}

func TestNewClientFromEnv_DefaultContext_FallsBack(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("DOCKER_CONTEXT", "")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	writeConfigCurrentContext(t, dir, "default")

	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if client.endpoint != defaultHost {
		t.Errorf("endpoint = %q, want default %q", client.endpoint, defaultHost)
	}
}
