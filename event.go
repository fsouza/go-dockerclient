package docker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/zenoss/go-dockerclient/utils"
)

// AllThingsDocker is a wildcard used to express interest in the Docker
// lifecycle event streams of all containers and images.
const AllThingsDocker = "*"

// Selectors for the various Docker lifecycle events.
const (
	Create  = "create"
	Delete  = "delete"
	Destroy = "destroy"
	Die     = "die"
	Export  = "export"
	Kill    = "kill"
	Restart = "restart"
	Start   = "start"
	Stop    = "stop"
	Untag   = "untag"
)

// EventMonitor implementations may be used to subscribe to Docker
// lifecycle events. This package provides such an implementation.
// Instances of it may be retreived via the client.EventMonitor() method.
type EventMonitor interface {
	// IsActive reports whether or not an EventMonitor is active, i.e., listening for Docker events.
	IsActive() bool

	// Subscribe returns a subscription to which handlers for the various Docker lifecycle events
	// for the container or image specified by ID (or all containers and images if AllThingsDocker
	// is passed) may be added.
	Subscribe(ID string) (*Subscription, error)

	// Close causes the EventMonitor to stop listening for Docker lifecycle events.
	Close() error
}

// Event represents a Docker lifecycle event.
type Event map[string]interface{}

// A HandlerFunc is used to receive Docker lifecycle events.
type HandlerFunc func(e Event) error

type clientEventMonitor struct {
	sync.Mutex
	active        bool
	closeChannel  chan chan struct{}
	done          chan struct{}
	subscriptions map[string][]chan Event
}

// Subscription represents a subscription to a particular container or image's Docker lifecycle
// event stream. The AllThingsDocker ID can be used to subscribe to all container and image
// event streams.
type Subscription struct {
	ID            string
	active        bool
	cancelChannel chan chan struct{}
	eventChannel  chan Event
	monitorDone   chan struct{}
	handlers      map[string]HandlerFunc
	monitor       *clientEventMonitor
}

// eventMonitor is used by the client to monitor Docker lifecycle events
var eventMonitor = &clientEventMonitor{
	active:        false,
	closeChannel:  make(chan chan struct{}),
	done:          make(chan struct{}),
	subscriptions: make(map[string][]chan Event),
}

// MonitorEvents returns an EventMonitor that can be used to listen for and respond to
// the various events in the Docker container and image lifecycles.
func (c *Client) MonitorEvents() (EventMonitor, error) {
	if err := eventMonitor.run(c); err != nil {
		return nil, err
	}

	return eventMonitor, nil
}

// IsActive reports whether or not an EventMonitor is active, i.e., listening for Docker events.
func (em *clientEventMonitor) IsActive() bool {
	em.Lock()
	defer em.Unlock()

	return em.active
}

// Close causes the EventMonitor to stop listening for Docker events.
func (em *clientEventMonitor) Close() error {
	em.Lock()
	defer em.Unlock()

	if !em.active {
		return nil
	}

	crc := make(chan struct{})
	em.closeChannel <- crc

	select {
	case <-crc:
		em.active = false
		em.subscriptions = make(map[string][]chan Event)
		em.done = make(chan struct{})
		return nil
	}

	return fmt.Errorf("unable to close %v", em)
}

// Subscribe returns a subscription to which handlers for the various Docker lifecycle events
// for the container or image specified by ID (or all containers and images if AllThingsDocker
// is passed) may be added.
func (em *clientEventMonitor) Subscribe(ID string) (*Subscription, error) {
	em.Lock()
	defer em.Unlock()

	s := &Subscription{
		ID:            ID,
		cancelChannel: make(chan chan struct{}),
		eventChannel:  make(chan Event),
		monitorDone:   em.done,
		handlers:      make(map[string]HandlerFunc),
		monitor:       em,
	}

	em.subscriptions[ID] = append(em.subscriptions[ID], s.eventChannel)
	s.run()

	return s, nil
}

// run causes the clientEventMonitor to start listening for Docker container
// and image lifecycle events
func (em *clientEventMonitor) run(c *Client) error {
	em.Lock()
	defer em.Unlock()

	if em.active {
		return nil
	}

	go func() {
		go listenAndDispatch(c, em)

		select {
		case crc := <-em.closeChannel:
			close(em.done)
			crc <- struct{}{}
			return
		}
	}()

	em.active = true
	return nil
}

// dispatch sends the incoming event to the event channel of all interested subscribers.
func (em *clientEventMonitor) dispatch(e string) error {
	em.Lock()
	defer em.Unlock()

	if !em.active {
		return nil
	}

	var evt Event

	err := json.Unmarshal([]byte(e), &evt)
	if err != nil {
		return err
	}

	// send the event to subscribers interested in everything
	if ecs, ok := em.subscriptions[AllThingsDocker]; ok {
		for _, ec := range ecs {
			ec <- evt
		}
	}

	// send the event to subscribers interested in the particular ID
	if ecs, ok := em.subscriptions[evt["id"].(string)]; ok {
		for _, ec := range ecs {
			ec <- evt
		}
	}

	return nil
}

// listenAndDispatch reads the Docker event stream and dispatches the events
// it receives.
func listenAndDispatch(c *Client, em *clientEventMonitor) {
	// TODO: figure out how to cleanly shutdown the hijacked connection and exit the scan loop.
	pr, pw := io.Pipe()

	go c.hijack("GET", "/events", true, nil, nil, pw)

	scanner := bufio.NewScanner(pr)
	for scanner.Scan() {
		et := scanner.Text()
		if et[0] == '{' {
			if err := em.dispatch(et); err != nil {
				utils.Debugf("unable to dispatch: %s (%v)", et, err)
			}
		}
	}
}

// Handle associates a HandlerFunc with a give Docker container or image lifecycle
// event. Any HandlerFunc previously associated the the specified event is replaced.
func (s *Subscription) Handle(es string, h HandlerFunc) error {
	s.handlers[es] = h
	return nil
}

// Cancel causes the Subscription to stop receiving and dispatching Docker container and
// image lifecycle events.
func (s *Subscription) Cancel() error {
	if !s.active {
		return nil
	}

	// remove this subscriber from the event monitor's subscription list
	// TODO: none event monitor things shouldn't manipulate its state, find a
	// better way to do this.
	ecs := s.monitor.subscriptions[s.ID]
	if len(ecs) == 1 {
		s.monitor.subscriptions[s.ID] = []chan Event{}
	} else {
		for i := 0; i < len(ecs); i++ {
			if ecs[i] == s.eventChannel {
				s.monitor.subscriptions[s.ID] = append(ecs[:i], ecs[i+1:]...)
			}
		}
	}

	crc := make(chan struct{})
	s.cancelChannel <- crc

	select {
	case <-crc:
		s.active = false
		return nil
	}

	return fmt.Errorf("unable to close %v", s)
}

// run causes the Subscription to start receiving and dispatching Docker container and
// image lifecycle events.
func (s *Subscription) run() error {
	if s.active {
		return nil
	}

	go func() {
		for {
			select {
			case e := <-s.eventChannel:
				if h, ok := s.handlers[e["status"].(string)]; ok {
					h(e)
				}
			case crc := <-s.cancelChannel:
				crc <- struct{}{}
				return
			case <-s.monitorDone:
				s.active = false
				return
			}
		}
	}()

	s.active = true
	return nil
}
