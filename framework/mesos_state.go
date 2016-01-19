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

var Mesos *MesosState

//TODO extend this struct when necessary
type MesosState struct {
	ActivatedSlaves float64 `json:"activated_slaves"`

	master string
	lock   sync.Mutex
}

func NewMesosState(master string) (*MesosState, error) {
	if !strings.HasPrefix(master, "http://") {
		master = "http://" + master
	}

	if strings.HasSuffix(master, "/") {
		master = master[:len(master)-1]
	}

	state := &MesosState{
		master: master,
	}

	return state, state.Update()
}

func (ms *MesosState) Update() error {
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

	return nil
}
