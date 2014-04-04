package docker

import (
	"testing"
	"time"
)

const (
	DockerEndpoint = "unix:///var/run/docker.sock"
)

func TestMonitorEvents(t *testing.T) {
	dc, err := NewClient(DockerEndpoint)
	if err != nil {
		t.Fatalf("can't create docker client: %v", err)
	}

	em, err := dc.MonitorEvents()
	if err != nil {
		t.Fatalf("can't create event monitor: %v", err)
	}

	status := em.Status()
	if status == nil {
		t.Fatal("nil event monitor status")
	}

	if !status.Active {
		t.Fatal("event monitor is inactive")
	}

	err = em.Close()
	if err != nil {
		t.Fatalf("can't close event monitor")
	}

	status = em.Status()
	if status == nil {
		t.Fatal("nil event monitor status")
	}

	if status.Active {
		t.Fatal("event monitor is active")
	}
}

func TestUniversalEventSubscription(t *testing.T) {
	dc, err := NewClient(DockerEndpoint)
	if err != nil {
		t.Fatalf("can't create docker client: %v", err)
	}

	em, err := dc.MonitorEvents()
	if err != nil {
		t.Fatalf("can't create event monitor: %v", err)
	}

	s, err := em.Subscribe(AllThingsDocker)
	if err != nil {
		t.Fatalf("universal subscription failed: %f", err)
	}

	cc := make(chan string)
	s.Handle(Create, func(e Event) error {
		cc <- e["status"].(string)
		return nil
	})

	c, err := dc.CreateContainer(CreateContainerOptions{"testues", &Config{
		Image: "ubuntu",
		Cmd:   []string{"/bin/bash"},
	}})
	if err != nil {
		t.Fatalf("couldn't create test container: %v", err)
	}

	to := time.After(10 * time.Second)
	select {
	case <-cc:
		t.Log("container got created")
	case <-to:
		t.Fatal("container creation timed out")
	}

	err = dc.StopContainer(c.ID, 10)
	if err != nil {
		t.Fatalf("unable to stop container %s: %v", c.ID, err)
	}

	err = dc.RemoveContainer(RemoveContainerOptions{ID: c.ID})
	if err != nil {
		t.Fatalf("unable to remove container %s: %v", c.ID, err)
	}

	err = em.Close()
	if err != nil {
		t.Fatal("can't close event monitor: %v", err)
	}
}

func TestContainerEventSubscription(t *testing.T) {
	dc, err := NewClient(DockerEndpoint)
	if err != nil {
		t.Fatalf("can't create docker client: %v", err)
	}

	em, err := dc.MonitorEvents()
	if err != nil {
		t.Fatalf("can't create event monitor: %v", err)
	}

	c, err := dc.CreateContainer(CreateContainerOptions{"testces", &Config{
		Image: "ubuntu",
		Cmd:   []string{"/bin/bash"},
	}})
	if err != nil {
		t.Fatalf("couldn't create test container: %v", err)
	}

	s, err := em.Subscribe(c.ID)
	if err != nil {
		t.Fatalf("subscription to container %v failed: %v", c.ID, err)
	}

	cc := make(chan string)

	s.Handle(Start, func(e Event) error {
		cc <- e["status"].(string)
		return nil
	})
	s.Handle(Stop, func(e Event) error {
		cc <- e["status"].(string)
		return nil
	})

	err = dc.StartContainer(c.ID, &HostConfig{})
	if err != nil {
		t.Fatalf("couldn't start test container: %v", err)
	}

	to := time.After(10 * time.Second)
	select {
	case <-cc:
		t.Log("test container started")
	case <-to:
		t.Fatal("start test container timed out")
	}

	err = dc.StopContainer(c.ID, 10)
	if err != nil {
		t.Fatalf("couldn't stop test container: %v", err)
	}

	to = time.After(10 * time.Second)
	select {
	case <-cc:
		t.Log("test container stopped")
	case <-to:
		t.Fatal("stop test container timed out")
	}

	err = dc.RemoveContainer(RemoveContainerOptions{ID: c.ID})
	if err != nil {
		t.Fatalf("unable to remove container %s: %v", c.ID, err)
	}

	err = em.Close()
	if err != nil {
		t.Fatalf("can't close event monitor: %v", err)
	}
}
