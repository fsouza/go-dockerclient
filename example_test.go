// Copyright 2013 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker_test

import (
	"archive/tar"
	"bytes"
	"github.com/fsouza/go-dockerclient"
	"io"
	"log"
)

func ExampleClient_AttachToContainer() {
	client, err := docker.NewClient("http://localhost:4243")
	if err != nil {
		log.Fatal(err)
	}
	// Reading logs from container a84849 and sending them to buf.
	var buf bytes.Buffer
	err = client.AttachToContainer(docker.AttachToContainerOptions{
		Container:    "a84849",
		OutputStream: &buf,
		Logs:         true,
		Stdout:       true,
		Stderr:       true,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println(buf.String())
	// Attaching to stdout and streaming.
	buf.Reset()
	err = client.AttachToContainer(docker.AttachToContainerOptions{
		Container:    "a84849",
		OutputStream: &buf,
		Stdout:       true,
		Stream:       true,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println(buf.String())
}

func ExampleClient_CopyFromContainer() {
	client, err := docker.NewClient("http://localhost:4243")
	if err != nil {
		log.Fatal(err)
	}
	cid := "a84849"
	// Copy resulting file
	var buf bytes.Buffer
	filename := "/tmp/output.txt"
	err = client.CopyFromContainer(docker.CopyFromContainerOptions{
		Container:    cid,
		Resource:     filename,
		OutputStream: &buf,
	})
	if err != nil {
		log.Fatalf("Error while copying from %s: %s\n", cid, err)
	}
	content := new(bytes.Buffer)
	r := bytes.NewReader(buf.Bytes())
	tr := tar.NewReader(r)
	tr.Next()
	if err != nil && err != io.EOF {
		log.Fatal(err)
	}
	if _, err := io.Copy(content, tr); err != nil {
		log.Fatal(err)
	}
	log.Println(buf.String())
}
