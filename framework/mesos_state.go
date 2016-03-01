/* Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. */

package framework

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
)

type MesosState interface {
	Update() error
	GetActivatedSlaves() int
	GetSlaves() []Slave
}

//TODO extend this struct when necessary
type MesosClusterState struct {
	ActivatedSlaves float64 `json:"activated_slaves"`
	Slaves          []Slave `json:"slaves"`

	master string
	lock   sync.Mutex
}

func NewMesosClusterState(master string) *MesosClusterState {
	if !strings.HasPrefix(master, "http://") {
		master = "http://" + master
	}

	if strings.HasSuffix(master, "/") {
		master = master[:len(master)-1]
	}

	return &MesosClusterState{
		master: master,
	}
}

func (ms *MesosClusterState) GetActivatedSlaves() int {
	return int(ms.ActivatedSlaves)
}

func (ms *MesosClusterState) GetSlaves() []Slave {
	return ms.Slaves
}

func (ms *MesosClusterState) Update() error {
	ms.lock.Lock()
	defer ms.lock.Unlock()

	url := ms.master + "/master/state.json"

	req, err := http.NewRequest("GET", url, nil)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, &ms)
	if err != nil {
		return err
	}

	Logger.Debug("Updated mesos state. New state: %s", ms)
	return nil
}

func (ms *MesosClusterState) String() string {
	js, err := json.MarshalIndent(ms, "", "  ")
	if err != nil {
		panic(err)
	}

	return string(js)
}

type Slave struct {
	Active     bool                   `json:"active"`
	Attributes map[string]string      `json:"attributes"`
	Hostname   string                 `json:"hostname"`
	ID         string                 `json:"id"`
	PID        string                 `json:"pid"`
	Resources  map[string]interface{} `json:"resources"`
}

func (s *Slave) Attribute(name string) string {
	if name == "hostname" {
		return s.Hostname
	}

	return s.Attributes[name]
}
