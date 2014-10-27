package docker

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"testing"

	"github.com/docker/docker/pkg/archive"
)

func TestBuildImageMultipleContextsError(t *testing.T) {
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	var buf bytes.Buffer
	opts := BuildImageOptions{
		Name:                "testImage",
		NoCache:             true,
		SuppressOutput:      true,
		RmTmpContainer:      true,
		ForceRmTmpContainer: true,
		InputStream:         &buf,
		OutputStream:        &buf,
		ContextDir:          "testing/data",
	}
	err := client.BuildImage(opts)
	if err != ErrMultipleContexts {
		t.Errorf("BuildImage: providing both InputStream and ContextDir should produce an error")
	}
}

func TestBuildImageContextDirDockerignoreParsing(t *testing.T) {
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	var buf bytes.Buffer
	opts := BuildImageOptions{
		Name:                "testImage",
		NoCache:             true,
		SuppressOutput:      true,
		RmTmpContainer:      true,
		ForceRmTmpContainer: true,
		OutputStream:        &buf,
		ContextDir:          "testing/data",
	}
	err := client.BuildImage(opts)
	if err != nil {
		t.Fatal(err)
	}
	reqBody := fakeRT.requests[0].Body
	tmpdir, err := unpackBodyTarball(reqBody)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := os.RemoveAll(tmpdir); err != nil {
			t.Fatal(err)
		}
	}()

	files, err := ioutil.ReadDir(tmpdir)
	if err != nil {
		t.Fatal(err)
	}

	foundFiles := []string{}
	for _, file := range files {
		foundFiles = append(foundFiles, file.Name())
	}

	expectedFiles := []string{".dockerignore", "Dockerfile", "barfile", "symlink"}

	if !reflect.DeepEqual(expectedFiles, foundFiles) {
		t.Errorf(
			"BuildImage: incorrect files sent in tarball to docker server\nexpected %+v, found %+v",
			expectedFiles, foundFiles,
		)
	}
}

func TestBuildImageSendXRegistryConfigFromEnv(t *testing.T) {
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	var buf bytes.Buffer
	opts := BuildImageOptions{
		Name:                "testImage",
		NoCache:             true,
		SuppressOutput:      true,
		RmTmpContainer:      true,
		ForceRmTmpContainer: true,
		OutputStream:        &buf,
		ContextDir:          "testing/data",
	}

	if err := os.Setenv("DOCKER_X_REGISTRY_CONFIG", "foobarbaz"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Setenv("DOCKER_X_REGISTRY_CONFIG", ""); err != nil {
			t.Fatal(err)
		}
	}()
	err := client.BuildImage(opts)
	if err != nil {
		t.Fatal(err)
	}
	if fakeRT.requests[0].Header["X-Registry-Config"][0] != "foobarbaz" {
		t.Errorf("BuildImage: X-Registry-Config not correctly set from the environment")
	}
}

func unpackBodyTarball(req io.ReadCloser) (tmpdir string, err error) {
	tmpdir, err = ioutil.TempDir("", "go-dockerclient-test")
	if err != nil {
		return
	}
	err = archive.Untar(req, tmpdir, &archive.TarOptions{
		Compression: archive.Uncompressed,
		NoLchown:    true,
	})
	return
}
