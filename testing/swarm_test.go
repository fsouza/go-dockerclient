// Copyright 2016 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/docker/engine-api/types/swarm"
	"github.com/fsouza/go-dockerclient"
)

func TestSwarmInit(t *testing.T) {
	server, _ := NewServer("127.0.0.1:0", nil, nil)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("POST", "/swarm/init", bytes.NewReader(nil))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("SwarmInit: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	var id string
	err := json.Unmarshal(recorder.Body.Bytes(), &id)
	if err != nil {
		t.Fatalf("SwarmInit: got error. %s", err.Error())
	}
	if id == "" {
		t.Fatal("SwarmInit: id not found")
	}
	if server.swarm == nil {
		t.Fatalf("SwarmInit: expected swarm to be set.")
	}
	if len(server.nodes) != 1 {
		t.Fatalf("SwarmInit: expected node len to be 1, got: %d", len(server.nodes))
	}
	if server.nodes[0].ManagerStatus.Addr != server.SwarmAddress() {
		t.Fatalf("SwarmInit: expected current node to have addr %q, got: %q", server.SwarmAddress(), server.nodes[0].ManagerStatus.Addr)
	}
	if !server.nodes[0].ManagerStatus.Leader {
		t.Fatalf("SwarmInit: expected current node to be leader")
	}
}

func TestSwarmInitDynamicAdvertiseAddrPort(t *testing.T) {
	server, _ := NewServer("127.0.0.1:0", nil, nil)
	server.buildMuxer()
	data := `{"ListenAddr": "127.0.0.1:0", "AdvertiseAddr": "localhost"}`
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("POST", "/swarm/init", strings.NewReader(data))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("SwarmInit: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	if len(server.nodes) != 1 {
		t.Fatalf("SwarmInit: expected node len to be 1, got: %d", len(server.nodes))
	}
	_, port, _ := net.SplitHostPort(server.SwarmAddress())
	expectedAddr := fmt.Sprintf("localhost:%s", port)
	if server.nodes[0].ManagerStatus.Addr != expectedAddr {
		t.Fatalf("SwarmInit: expected current node to have addr %q, got: %q", expectedAddr, server.nodes[0].ManagerStatus.Addr)
	}
}

func TestSwarmInitAlreadyInSwarm(t *testing.T) {
	server, _ := NewServer("127.0.0.1:0", nil, nil)
	server.buildMuxer()
	server.swarm = &swarm.Swarm{}
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("POST", "/swarm/init", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotAcceptable {
		t.Fatalf("SwarmInit: wrong status. Want %d. Got %d.", http.StatusNotAcceptable, recorder.Code)
	}
}

func TestSwarmJoinNoBody(t *testing.T) {
	server, _ := NewServer("127.0.0.1:0", nil, nil)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("POST", "/swarm/join", bytes.NewReader(nil))
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("SwarmJoin: wrong status. Want %d. Got %d.", http.StatusInternalServerError, recorder.Code)
	}
	if server.swarm != nil {
		t.Fatalf("SwarmJoin: expected swarm not to be set.")
	}
}

func TestSwarmJoin(t *testing.T) {
	server1, _ := NewServer("127.0.0.1:0", nil, nil)
	server2, _ := NewServer("127.0.0.1:0", nil, nil)
	data, err := json.Marshal(swarm.InitRequest{})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("POST", "/swarm/init", bytes.NewReader(data))
	server1.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("SwarmJoin: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	data, err = json.Marshal(swarm.JoinRequest{
		RemoteAddrs: []string{server1.SwarmAddress()},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder = httptest.NewRecorder()
	request, _ = http.NewRequest("POST", "/swarm/join", bytes.NewReader(data))
	server2.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("SwarmJoin: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	if server1.swarm == nil || server2.swarm == nil {
		t.Fatalf("SwarmJoin: expected swarm to be set.")
	}
	if len(server1.nodes) != 2 {
		t.Fatalf("SwarmJoin: expected node len to be 2, got: %d", len(server1.nodes))
	}
	if server1.nodes[0].ManagerStatus.Addr != server1.SwarmAddress() {
		t.Fatalf("SwarmJoin: expected nodes[0] to have addr %q, got: %q", server1.SwarmAddress(), server1.nodes[0].ManagerStatus.Addr)
	}
	if server1.nodes[1].ManagerStatus.Leader {
		t.Fatalf("SwarmInit: expected nodes[1] not to be leader")
	}
	if server1.nodes[1].ManagerStatus.Addr != server2.SwarmAddress() {
		t.Fatalf("SwarmJoin: expected nodes[1] to have addr %q, got: %q", server2.SwarmAddress(), server1.nodes[1].ManagerStatus.Addr)
	}
	if !reflect.DeepEqual(server1.nodes, server2.nodes) {
		t.Fatalf("SwarmJoin: expected nodes to be equal in server1 and server2, got:\n%#v\n%#v", server1.nodes, server2.nodes)
	}
}

func TestSwarmJoinAlreadyInSwarm(t *testing.T) {
	server, _ := NewServer("127.0.0.1:0", nil, nil)
	server.buildMuxer()
	server.swarm = &swarm.Swarm{}
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("POST", "/swarm/join", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotAcceptable {
		t.Fatalf("SwarmJoin: wrong status. Want %d. Got %d.", http.StatusNotAcceptable, recorder.Code)
	}
}

func TestSwarmLeave(t *testing.T) {
	server, _ := NewServer("127.0.0.1:0", nil, nil)
	server.buildMuxer()
	server.swarm = &swarm.Swarm{}
	server.swarmServer, _ = newSwarmServer(server, "127.0.0.1:0")
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("POST", "/swarm/leave", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("SwarmLeave: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	if server.swarm != nil {
		t.Fatalf("SwarmLeave: expected swarm to be nil. Got %+v.", server.swarm)
	}
}

func TestSwarmLeaveNotInSwarm(t *testing.T) {
	server, _ := NewServer("127.0.0.1:0", nil, nil)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("POST", "/swarm/leave", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotAcceptable {
		t.Fatalf("SwarmLeave: wrong status. Want %d. Got %d.", http.StatusNotAcceptable, recorder.Code)
	}
	if server.swarm != nil {
		t.Fatalf("SwarmLeave: expected swarm to be nil. Got %+v.", server.swarm)
	}
}

func TestSwarmInspect(t *testing.T) {
	server, _ := NewServer("127.0.0.1:0", nil, nil)
	server.buildMuxer()
	expected := &swarm.Swarm{
		ClusterInfo: swarm.ClusterInfo{
			ID: "swarm-id",
		},
	}
	server.swarm = expected
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("GET", "/swarm", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("SwarmInspect: wrong status. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	var swarmInspect *swarm.Swarm
	err := json.Unmarshal(recorder.Body.Bytes(), &swarmInspect)
	if err != nil {
		t.Fatalf("SwarmInspect: got error. %s", err.Error())
	}
	if expected.ClusterInfo.ID != swarmInspect.ClusterInfo.ID {
		t.Fatalf("SwarmInspect: wrong response. Want %+v. Got %+v.", expected, swarmInspect)
	}
}

func TestSwarmInspectNotInSwarm(t *testing.T) {
	server, _ := NewServer("127.0.0.1:0", nil, nil)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("GET", "/swarm", nil)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotAcceptable {
		t.Fatalf("SwarmInspect: wrong status. Want %d. Got %d.", http.StatusNotAcceptable, recorder.Code)
	}
}

func TestServiceCreate(t *testing.T) {
	server, _ := NewServer("127.0.0.1:0", nil, nil)
	server.buildMuxer()
	recorder := httptest.NewRecorder()
	serviceCreateOpts := docker.CreateServiceOptions{
		ServiceSpec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{
				Name: "test",
			},
			TaskTemplate: swarm.TaskSpec{
				ContainerSpec: swarm.ContainerSpec{
					Image: "test/test",
					Args:  []string{"--test"},
					Env:   []string{"ENV=1"},
					User:  "test",
				},
			},
			EndpointSpec: &swarm.EndpointSpec{
				Mode: swarm.ResolutionModeVIP,
				Ports: []swarm.PortConfig{{
					Protocol:      swarm.PortConfigProtocolTCP,
					TargetPort:    uint32(80),
					PublishedPort: uint32(80),
				}},
			},
		},
	}
	buf, err := json.Marshal(serviceCreateOpts)
	if err != nil {
		t.Fatalf("ServiceCreate error: %s", err.Error())
	}
	var params io.Reader
	params = bytes.NewBuffer(buf)
	request, _ := http.NewRequest("POST", "/services/create", params)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("ServiceCreate: wrong status code. Want %d. Got %d.", http.StatusOK, recorder.Code)
	}
	if len(server.containers) != 1 {
		t.Fatal("ServiceCreate: service not started")
	}
	expectedContainer := docker.Container{
		Image: "test/test",
		Name:  "test",
		Config: &docker.Config{
			Cmd:          []string{"--test"},
			Env:          []string{"ENV=1"},
			ExposedPorts: map[docker.Port]struct{}{"80/tcp": {}},
		},
		HostConfig: &docker.HostConfig{
			PortBindings: map[docker.Port][]docker.PortBinding{
				"80/tcp": {
					{HostIP: "0.0.0.0", HostPort: "80"},
				},
			},
		},
	}
	cont := server.containers[0]
	if cont.Config.Env == nil {
		t.Fatalf("ServiceCreate: wrong conf. Want %#v. Got %#v.", expectedContainer.Config.Env, cont.Config.Env)
	}
	if cont.Config.ExposedPorts == nil {
		t.Fatalf("ServiceCreate: wrong conf. Want %#v. Got %#v.", expectedContainer.Config.ExposedPorts, cont.Config.ExposedPorts)
	}
	if cont.Config.Cmd == nil {
		t.Fatalf("ServiceCreate: wrong conf. Want %#v. Got %#v.", expectedContainer.Config.Cmd, cont.Config.Cmd)
	}
	if cont.Name != expectedContainer.Name {
		t.Fatalf("ServiceCreate: wrong conf. Want %#v. Got %#v.", expectedContainer.Name, cont.Name)
	}
	if cont.Image != expectedContainer.Image {
		t.Fatalf("ServiceCreate: wrong conf. Want %#v. Got %#v.", expectedContainer.Image, cont.Image)
	}
	if cont.HostConfig.PortBindings == nil {
		t.Fatalf("ServiceCreate: wrong conf. Want %#v. Got %#v.", expectedContainer.HostConfig.PortBindings, cont.HostConfig.PortBindings)
	}
}

func TestNodeList(t *testing.T) {
	srv1, srv2, err := setUpSwarm()
	if err != nil {
		t.Fatal(err)
	}
	for _, srv := range []*DockerServer{srv1, srv2} {
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest("GET", "/nodes", nil)
		srv.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("invalid status code: %d", recorder.Code)
		}
		var nodes []swarm.Node
		err = json.NewDecoder(recorder.Body).Decode(&nodes)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(nodes, srv1.nodes) {
			t.Fatalf("expected nodes to equal %#v, got: %#v", srv1.nodes, nodes)
		}
		if !reflect.DeepEqual(nodes, srv2.nodes) {
			t.Fatalf("expected nodes to equal %#v, got: %#v", srv2.nodes, nodes)
		}
	}
}

func TestNodeInfo(t *testing.T) {
	srv1, srv2, err := setUpSwarm()
	if err != nil {
		t.Fatal(err)
	}
	for _, srv := range []*DockerServer{srv1, srv2} {
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest("GET", "/nodes/"+srv.nodes[0].ID, nil)
		srv.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("invalid status code: %d", recorder.Code)
		}
		var node swarm.Node
		err = json.NewDecoder(recorder.Body).Decode(&node)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(node, srv1.nodes[0]) {
			t.Fatalf("expected node to equal %#v, got: %#v", srv1.nodes[0], node)
		}
		if !reflect.DeepEqual(node, srv2.nodes[0]) {
			t.Fatalf("expected node to equal %#v, got: %#v", srv2.nodes[0], node)
		}
	}
}

func TestNodeUpdate(t *testing.T) {
	srv1, srv2, err := setUpSwarm()
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	for i, srv := range []*DockerServer{srv1, srv2} {
		key := fmt.Sprintf("l%d", i)
		data, err := json.Marshal(swarm.NodeSpec{
			Annotations: swarm.Annotations{Labels: map[string]string{key: "value"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		body := bytes.NewReader(data)
		request, _ := http.NewRequest("POST", "/nodes/"+srv.nodes[0].ID+"/update", body)
		srv.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("invalid status code: %d", recorder.Code)
		}
		if srv1.nodes[0].Spec.Annotations.Labels[key] != "value" {
			t.Fatalf("expected node to have label %s", key)
		}
		if srv2.nodes[0].Spec.Annotations.Labels[key] != "value" {
			t.Fatalf("expected node to have label %s", key)
		}
	}
}

func TestNodeDelete(t *testing.T) {
	srv1, srv2, err := setUpSwarm()
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("DELETE", "/nodes/"+srv1.nodes[0].ID, nil)
	srv1.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("invalid status code: %d", recorder.Code)
	}
	if len(srv1.nodes) != 1 {
		t.Fatalf("expected len(nodes) to be 1, got %d", len(srv1.nodes))
	}
	if len(srv2.nodes) != 1 {
		t.Fatalf("expected len(nodes) to be 1, got %d", len(srv2.nodes))
	}
}

func setUpSwarm() (*DockerServer, *DockerServer, error) {
	server1, _ := NewServer("127.0.0.1:0", nil, nil)
	server2, _ := NewServer("127.0.0.1:0", nil, nil)
	recorder := httptest.NewRecorder()
	request, _ := http.NewRequest("POST", "/swarm/init", bytes.NewReader(nil))
	server1.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		return nil, nil, fmt.Errorf("invalid status code %d", recorder.Code)
	}
	data, err := json.Marshal(swarm.JoinRequest{
		RemoteAddrs: []string{server1.SwarmAddress()},
	})
	if err != nil {
		return nil, nil, err
	}
	recorder = httptest.NewRecorder()
	request, _ = http.NewRequest("POST", "/swarm/join", bytes.NewReader(data))
	server2.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		return nil, nil, fmt.Errorf("invalid status code %d", recorder.Code)
	}
	return server1, server2, nil
}
