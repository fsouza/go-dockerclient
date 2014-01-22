#go-dockerclient

[![Build Status](https://drone.io/github.com/fsouza/go-dockerclient/status.png)](https://drone.io/github.com/fsouza/go-dockerclient/latest)
[![Build Status](https://travis-ci.org/fsouza/go-dockerclient.png)](https://travis-ci.org/fsouza/go-dockerclient)

[![GoDoc](http://godoc.org/github.com/fsouza/go-dockerclient?status.png)](http://godoc.org/github.com/fsouza/go-dockerclient)

This package presents a client for the Docker remote API.

For more details, check the remote API documentation:
http://docs.docker.io/en/latest/api/docker_remote_api.

##Versioning

* Version 0.1 is compatible with Docker v0.7.1
* The master is compatible with Docker's master


## Example

     package main

     import (
       	    "fmt"
	    "github.com/fsouza/go-dockerclient"
     )

    func main() {
	    images := &Images{}
	    endpoint := "unix:///var/run/docker.sock"
	    client, _ := docker.NewClient(endpoint)
	    imgs, _ := client.ListImages(true)
	    for _, img := range imgs {
		    fmt.Println("ID:", img.ID)
		    fmt.Println("Virtual Size:", img.VirtualSize)
		    fmt.Println("RepoTag:", img.RepoTag)
		    fmt.Println("Repository:", img.Repository)
	   	    fmt.Println("Tag:", img.Tag)
		    fmt.Println("Created:", img.Created)
		    fmt.Println("Size:", img.Size)
		    images.Add(img)
	    }
    }

    type Images struct {
	    list []docker.Image
	    Len  int
    }

    func (images *Images) Add(i docker.Image) {
	    images.list = append(images.list, i)
	    images.Len = len(images.list)
    }
