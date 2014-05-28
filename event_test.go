package docker

import (
	"sync"
	"testing"
	"time"

	"github.com/dotcloud/docker/utils"
)

const (
	DockerEndpoint = "http://127.0.0.1:3006"
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

	if !em.IsActive() {
		t.Fatal("event monitor is inactive")
	}

	err = em.Close()
	if err != nil {
		t.Fatalf("can't close event monitor")
	}

	if em.IsActive() {
		t.Fatal("event monitor is active")
	}
}

func TestUnknownEventHandler(t *testing.T) {
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

	err = s.Handle("BooYa!", func(e Event) error {
		return nil
	})
	if err == nil {
		t.Fatal("expecting unknown event error")
	}

	em.Close()
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
	s.Handle(Destroy, func(e Event) error {
		utils.Debugf("handling destruction")
		cc <- e["status"].(string)
		return nil
	})

	c, err := dc.CreateContainer(CreateContainerOptions{"", &Config{
		Image:        "ubuntu",
		Cmd:          []string{"/bin/bash"},
		AttachStdout: true,
	}})
	if err != nil {
		t.Fatalf("couldn't create test container: %v", err)
	}

	to := time.After(10 * time.Second)
	select {
	case <-cc:
		t.Log("container created")
	case <-to:
		t.Fatal("container creation timed out")
	}

	err = dc.RemoveContainer(RemoveContainerOptions{ID: c.ID})
	if err != nil {
		t.Fatalf("unable to remove container %s: %v", c.ID, err)
	}

	to = time.After(10 * time.Second)
	select {
	case <-cc:
		t.Log("container removed")
	case <-to:
		t.Fatal("container removal timed out")
	}

	err = s.Cancel()
	if err != nil {
		t.Fatal("can't close subscription: %v", err)
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

	c, err := dc.CreateContainer(CreateContainerOptions{"", &Config{
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
	s.Handle(Die, func(e Event) error {
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

	to = time.After(10 * time.Second)
	select {
	case <-cc:
		t.Log("test container died")
	case <-to:
		t.Fatal("timed out waiting for container to die")
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

func TestCombinedEventSubscription(t *testing.T) {
	dc, err := NewClient(DockerEndpoint)
	if err != nil {
		t.Fatalf("can't create docker client: %v", err)
	}

	em, err := dc.MonitorEvents()
	if err != nil {
		t.Fatalf("can't create event monitor: %v", err)
	}

	sa, err := em.Subscribe(AllThingsDocker)
	if err != nil {
		t.Fatalf("universal subscription failed: %f", err)
	}

	sac := make(chan string)
	sa.Handle(Create, func(e Event) error {
		sac <- e["status"].(string)
		return nil
	})
	sa.Handle(Destroy, func(e Event) error {
		utils.Debugf("handling destruction")
		sac <- e["status"].(string)
		return nil
	})

	c, err := dc.CreateContainer(CreateContainerOptions{"", &Config{
		Image: "ubuntu",
		Cmd:   []string{"/bin/bash"},
	}})
	if err != nil {
		t.Fatalf("couldn't create test container: %v", err)
	}

	to := time.After(10 * time.Second)
	select {
	case <-sac:
		t.Log("container created")
	case <-to:
		t.Fatal("container creation timed out")
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
	s.Handle(Die, func(e Event) error {
		cc <- e["status"].(string)
		return nil
	})

	err = dc.StartContainer(c.ID, &HostConfig{})
	if err != nil {
		t.Fatalf("couldn't start test container: %v", err)
	}

	to = time.After(10 * time.Second)
	select {
	case <-cc:
		t.Log("test container started")
	case <-to:
		t.Fatal("start test container timed out")
	}

	to = time.After(10 * time.Second)
	select {
	case <-cc:
		t.Log("test container died")
	case <-to:
		t.Fatal("timed out waiting for container to die")
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

func TestEventMonitorSubscriptionCancelation(t *testing.T) {
	dc, err := NewClient(DockerEndpoint)
	if err != nil {
		t.Fatalf("can't create docker client: %v", err)
	}

	em, err := dc.MonitorEvents()
	if err != nil {
		t.Fatalf("can't create event monitor: %v", err)
	}

	var wg sync.WaitGroup

	subscriptions := []*Subscription{}
	for _, n := range []int{1, 2, 3, 4, 5} {
		s, err := em.Subscribe(AllThingsDocker)
		if err != nil {
			t.Fatal("subscription %d failed: %v", n, err)
		}
		subscriptions = append(subscriptions, s)
		wg.Add(1)
	}

	hf := func(e Event) error {
		wg.Done()
		return nil
	}

	for _, s := range subscriptions {
		s.Handle(Create, hf)
	}

	c, err := dc.CreateContainer(CreateContainerOptions{"", &Config{
		Image: "ubuntu",
		Cmd:   []string{"/bin/bash"},
	}})
	if err != nil {
		t.Fatalf("couldn't create test container: %v", err)
	}

	wg.Wait()

	err = dc.RemoveContainer(RemoveContainerOptions{ID: c.ID})
	if err != nil {
		t.Fatalf("unable to remove container %s: %v", c.ID, err)
	}

	em.Close()
}
