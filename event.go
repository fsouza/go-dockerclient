// Copyright 2013 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
	"time"
)

type APIEvents struct {
	Status string
	ID     string
	From   string
	Time   int64
}

type EventMonitoringState struct {
	sync.RWMutex
	enabled   bool
	lastSeen  int64
	C         chan *APIEvents
	errC      chan error
	listeners []chan *APIEvents
}

var eventMonitor EventMonitoringState
var ErrNoListeners = errors.New("No listeners to send event to...")

func (c *Client) AddEventListener(listener chan *APIEvents) error {
	err := eventMonitor.enableEventMonitoring(c)
	if err != nil {
		return err
	}
	err = eventMonitor.addListener(listener)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) RemoveEventListener(listener chan *APIEvents) error {
	err := eventMonitor.removeListener(listener)
	if err != nil {
		return err
	}

	if len(eventMonitor.listeners) == 0 {
		err = eventMonitor.disableEventMonitoring()
		if err != nil {
			return err
		}
	}

	return nil
}

func (eventState *EventMonitoringState) addListener(listener chan *APIEvents) error {
	eventState.Lock()
	defer eventState.Unlock()
	if listenerExists(listener, &eventState.listeners) {
		return fmt.Errorf("Listener already exists")
	}
	eventState.listeners = append(eventState.listeners, listener)
	return nil
}

func (eventState *EventMonitoringState) removeListener(listener chan *APIEvents) error {
	eventState.Lock()
	defer eventState.Unlock()
	var newListeners []chan *APIEvents
	if listenerExists(listener, &eventState.listeners) {
		for _, l := range eventState.listeners {
			if l != listener {
				newListeners = append(newListeners, l)
			}
		}
		eventState.listeners = newListeners
	}
	return nil
}

func listenerExists(a chan *APIEvents, list *[]chan *APIEvents) bool {
	for _, b := range *list {
		if b == a {
			return true
		}
	}
	return false
}

func (eventState *EventMonitoringState) enableEventMonitoring(c *Client) error {
	eventState.Lock()
	defer eventState.Unlock()
	if !eventState.enabled {
		eventState.enabled = true
		eventState.C = make(chan *APIEvents, 100)
		eventState.errC = make(chan error, 1)
		go eventState.monitorEvents(c)
	}
	return nil
}

func (eventState *EventMonitoringState) disableEventMonitoring() error {
	eventState.Lock()
	defer eventState.Unlock()
	if !eventState.enabled {
		eventState.enabled = false
		close(eventState.C)
		close(eventState.errC)
	}
	return nil
}

func (eventState *EventMonitoringState) monitorEvents(c *Client) {
	var retries int
	var err error

	// wait for first listener
	for len(eventState.listeners) == 0 {
		time.Sleep(10 * time.Millisecond)
	}

	for err = c.eventHijack(uint32(eventState.lastSeen), eventState.C, eventState.errC); err != nil && retries < 5; retries++ {
		waitTime := int64(float64(10) * math.Pow(2, float64(retries)))
		time.Sleep(time.Duration(waitTime) * time.Millisecond)
		err = c.eventHijack(uint32(eventState.lastSeen), eventState.C, eventState.errC)
	}

	if err != nil {
		eventState.terminate(err)
	}

	for eventState.enabled {
		timeout := time.After(100 * time.Millisecond)
		select {
		case ev := <-eventState.C:
			// send the event
			go eventState.sendEvent(ev)

			// update lastSeen if appropriate
			go func(e *APIEvents) {
				eventState.Lock()
				defer eventState.Unlock()
				if eventState.lastSeen < e.Time {
					eventState.lastSeen = e.Time
				}
			}(ev)

		case err = <-eventState.errC:
			if err == ErrNoListeners {
				// if there are no listeners, exit normally
				eventState.terminate(nil)
				return
			} else if err != nil {
				// otherwise, trigger a restart via the error channel
				defer func() { go eventState.monitorEvents(c) }()
				return
			}
		case <-timeout:
			continue
		}
	}
}

func (eventState *EventMonitoringState) sendEvent(event *APIEvents) {

	eventState.RLock()
	defer eventState.RUnlock()
	if len(eventState.listeners) == 0 {
		eventState.errC <- ErrNoListeners
	}
	for _, listener := range eventState.listeners {
		listener <- event
	}
}

func (eventState *EventMonitoringState) terminate(err error) {
	eventState.disableEventMonitoring()
}

func (c *Client) eventHijack(startTime uint32, eventChan chan *APIEvents, errChan chan error) error {

	uri := "/events"

	if startTime != 0 {
		uri += fmt.Sprintf("?since=%d", startTime)
	}

	req, err := http.NewRequest("GET", c.getURL(uri), nil)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "plain/text")
	protocol := c.endpointURL.Scheme
	address := c.endpointURL.Path
	if protocol != "unix" {
		protocol = "tcp"
		address = c.endpointURL.Host
	}

	dial, err := net.Dial(protocol, address)
	if err != nil {
		return err
	}

	clientconn := httputil.NewClientConn(dial, nil)
	clientconn.Do(req)

	conn, rwc := clientconn.Hijack()
	if err != nil {
		return err
	}

	go func(rwc io.Reader) {

		defer clientconn.Close()
		defer conn.Close()

		scanner := bufio.NewScanner(rwc)
		for scanner.Scan() {
			line := scanner.Text()

			// Only pay attention to lines that start as json objects
			if strings.HasPrefix(line, "{") {
				var e APIEvents
				err = json.Unmarshal([]byte(line), &e)
				if err != nil {
					errChan <- err
				}
				eventChan <- &e
			}

		}
		if err := scanner.Err(); err != nil {
			errChan <- err
		}
	}(rwc)

	return nil
}
