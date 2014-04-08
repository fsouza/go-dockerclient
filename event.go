package docker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

const (
	AllThingsDocker = "*"
	Create          = "create"
	Delete          = "delete"
	Destroy         = "destroy"
	Die             = "die"
	Export          = "export"
	Kill            = "kill"
	Restart         = "restart"
	Start           = "start"
	Stop            = "stop"
	Untag           = "untag"
)

type EventMonitor interface {
	IsActive() bool
	Subscribe(ID string) (*Subscription, error)
	Close() error
}

type Event map[string]interface{}
type HandlerFunc func(e Event) error

type clientEventMonitor struct {
	sync.Mutex
	active        bool
	closeChannel  chan chan struct{}
	subscriptions map[string][]chan Event
}

type Subscription struct {
	ID           string
	active       bool
	closeChannel chan chan struct{}
	eventChannel chan Event
	handlers     map[string]HandlerFunc
}

// eventMonitor is used by the client to monitor Docker lifecycle events
var eventMonitor = &clientEventMonitor{
	active:        false,
	closeChannel:  make(chan chan struct{}),
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
		return nil
	}

	return fmt.Errorf("unable to close %v", em)
}

// Subscribe returns a subscription to which handlers for the various events
// in the Docker container and image lifecycles may be added.
func (em *clientEventMonitor) Subscribe(ID string) (*Subscription, error) {
	em.Lock()
	defer em.Unlock()

	s := &Subscription{
		ID:           ID,
		closeChannel: make(chan chan struct{}),
		eventChannel: make(chan Event),
		handlers:     make(map[string]HandlerFunc),
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
	pr, pw := io.Pipe()

	go c.Hijack("GET", "/events", true, nil, nil, pw)

	scanner := bufio.NewScanner(pr)
	for scanner.Scan() {
		et := scanner.Text()
		if et[0] == '{' {
			_ = em.dispatch(et)
		}
	}

	panic("here are the stacks")
}

// Handle associates a HandlerFunc with a give Docker container or image lifecycle
// event. Any HandlerFunc previously associated the the specified event is replaced.
func (s *Subscription) Handle(es string, h HandlerFunc) error {
	s.handlers[es] = h
	return nil
}

// Close causes the Subscription to stop receiving and dispatching Docker container and
// image lifecycle events.
func (s *Subscription) Close() error {
	if !s.active {
		return nil
	}

	crc := make(chan struct{})
	s.closeChannel <- crc

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
		select {
		case e := <-s.eventChannel:
			fmt.Printf("received: %v\n", e)
			if h, ok := s.handlers[e["status"].(string)]; ok {
				fmt.Printf("dispatching to %v\n", h)
				h(e)
			}
		case crc := <-s.closeChannel:
			crc <- struct{}{}
			return
		}
	}()

	s.active = true
	return nil
}
