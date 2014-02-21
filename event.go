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
	"os"
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
	fmt.Println("enter AddEventListener")
	defer fmt.Println("exit AddEventListener")
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
	fmt.Println("enter RemoveEventListener")
	defer fmt.Println("exit RemoveEventListener")
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
	fmt.Println("enter addListener")
	defer fmt.Println("exit addListener")

	eventState.Lock()
	defer eventState.Unlock()
	if listenerExists(listener, &eventState.listeners) {
		return fmt.Errorf("Listener already exists")
	}
	eventState.listeners = append(eventState.listeners, listener)
	return nil
}

func (eventState *EventMonitoringState) removeListener(listener chan *APIEvents) error {
	fmt.Println("enter removeListener")
	defer fmt.Println("exit removeListener")
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
	fmt.Println("enter listenerExists")
	defer fmt.Println("exit listenerExists")
	for _, b := range *list {
		if b == a {
			return true
		}
	}
	return false
}

func (eventState *EventMonitoringState) enableEventMonitoring(c *Client) error {
	fmt.Println("enter enableEventMonitoring")
	defer fmt.Println("exit enableEventMonitoring")
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
	fmt.Println("enter disableEventMonitoring")
	defer fmt.Println("exit disableEventMonitoring")
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
	fmt.Println("enter monitorEvents")
	defer fmt.Println("exit monitorEvents")
	var retries int
	var err error

	// wait for first listener
	for len(eventState.listeners) == 0 {
		time.Sleep(10 * time.Millisecond)
	}

	for err = c.eventHijack(uint32(eventState.lastSeen), eventState.C, eventState.errC); err != nil && retries < 5; retries++ {
		fmt.Printf("eventHijack retry: %s\n", err)
		waitTime := int64(float64(10) * math.Pow(2, float64(retries)))
		fmt.Printf("connection to event stream failed, retrying in %n ms: %s", waitTime, err)
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
			fmt.Println("monitorEvents.C recieved")
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
			fmt.Println("monitorEvents errC recieved")
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
			fmt.Println("monitorEvents timeout")
			continue
		}
	}
}

func (eventState *EventMonitoringState) sendEvent(event *APIEvents) {
	fmt.Println("enter sendEvent")
	defer fmt.Println("exit sendEvent")

	eventState.RLock()
	defer eventState.RUnlock()
	fmt.Printf("sending to %n listeners\n", len(eventState.listeners))
	if len(eventState.listeners) == 0 {
		eventState.errC <- ErrNoListeners
	}
	for _, listener := range eventState.listeners {
		listener <- event
	}
}

func (eventState *EventMonitoringState) terminate(err error) {
	fmt.Println("enter terminate")
	defer fmt.Println("exit terminate")
	if err != nil {
		fmt.Printf("terminating montoring", err)
	}
	eventState.disableEventMonitoring()
}

func (c *Client) eventHijack(startTime uint32, eventChan chan *APIEvents, errChan chan error) error {
	fmt.Println("enter eventHijack")
	defer fmt.Println("exit eventHijack")

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

	rwc, _ := clientconn.Hijack()

	fmt.Printf("remote: %s\n", rwc.LocalAddr().String())

	go func(rwc io.ReadWriteCloser) {
		fmt.Println("enter eventHijack goroutine")
		defer fmt.Println("exit eventHijack goroutine")

		defer clientconn.Close()
		defer rwc.Close()

		scanner := bufio.NewScanner(rwc)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Printf("rwc.RCV: %s\n", line)

			var e APIEvents
			err = json.Unmarshal([]byte(line), &e)
			if err != nil {
				errChan <- err
			}

		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "reading from network:", err)
		}
	}(rwc)

	return nil
}
