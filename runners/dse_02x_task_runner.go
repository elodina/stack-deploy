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

package runners

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/elodina/stack-deploy/framework"
	"github.com/gambol99/go-marathon"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

type DSE02xTaskRunner struct{}

func (dtr *DSE02xTaskRunner) FillContext(context *framework.Context, application *framework.Application, task marathon.Task) error {
	context.Set(fmt.Sprintf("%s.host", application.ID), task.Host)
	for idx, port := range task.Ports {
		context.Set(fmt.Sprintf("%s.port%d", application.ID, idx), fmt.Sprint(port))
	}
	context.Set(fmt.Sprintf("%s.api", application.ID), fmt.Sprintf("http://%s:%d", task.Host, task.Ports[0]))

	return nil
}

func (dtr *DSE02xTaskRunner) RunTask(context *framework.Context, application *framework.Application, task map[string]string) error {
	api := context.MustGet(fmt.Sprintf("%s.api", application.ID))

	client := NewDSEMesos02xClient(api)
	_, err := client.AddCluster(task)
	if err != nil {
		return err
	}

	_, err = client.Add(task)
	if err != nil {
		return err
	}

	response, err := client.Start(task)
	if err != nil {
		return err
	}

	return dtr.fillTaskContext(context, application, response)
}

func (dtr *DSE02xTaskRunner) fillTaskContext(context *framework.Context, application *framework.Application, response []byte) error {
	startResponse := new(DSEMesos02xStartResponse)
	err := json.Unmarshal(response, &startResponse)
	if err != nil {
		return err
	}

	if startResponse.Status == "timeout" {
		return errors.New("Timed out")
	}

	servers := make([]string, 0)
	for _, node := range startResponse.Nodes {
		nodeEndpoint := fmt.Sprintf("%s:%d", node.Runtime.Address, node.Runtime.Reservation.Ports["cql"])

		context.Set(fmt.Sprintf("%s.cassandra-%s", application.ID, fmt.Sprint(node.ID)), nodeEndpoint)
		context.Set(fmt.Sprintf("%s.cassandra-%s.host", application.ID, fmt.Sprint(node.ID)), node.Runtime.Address)
		for name, port := range node.Runtime.Reservation.Ports {
			context.Set(fmt.Sprintf("%s.cassandra-%s.%sPort", application.ID, fmt.Sprint(node.ID), name), fmt.Sprint(port))
		}

		servers = append(servers, nodeEndpoint)
	}

	context.Set(fmt.Sprintf("%s.cassandraConnect", application.ID), strings.Join(servers, ","))
	return nil
}

type DSEMesos02xClient struct {
	api string
}

func NewDSEMesos02xClient(api string) *DSEMesos02xClient {
	return &DSEMesos02xClient{
		api: api,
	}
}

func (dmc *DSEMesos02xClient) AddCluster(params map[string]string) ([]byte, error) {
	Logger.Info("Adding cluster %s", params["cluster"])

	return dmc.doGet(fmt.Sprintf("%s/api/cluster/add", dmc.api), params)
}

func (dmc *DSEMesos02xClient) Add(params map[string]string) ([]byte, error) {
	Logger.Info("Adding %s", params["node"])

	return dmc.doGet(fmt.Sprintf("%s/api/node/add", dmc.api), params)
}

func (dmc *DSEMesos02xClient) Start(params map[string]string) ([]byte, error) {
	Logger.Info("Starting %s", params["node"])

	return dmc.doGet(fmt.Sprintf("%s/api/node/start", dmc.api), params)
}

func (dmc *DSEMesos02xClient) doGet(requestUrl string, params map[string]string) ([]byte, error) {
	getParams := url.Values{}
	for k, v := range params {
		getParams.Add(k, v)
	}

	rawResponse, err := http.Get(fmt.Sprintf("%s?%s", requestUrl, getParams.Encode()))
	if err != nil {
		return nil, err
	}

	return dmc.checkResponse(rawResponse)
}

func (dmc *DSEMesos02xClient) checkResponse(rawResponse *http.Response) ([]byte, error) {
	body, err := ioutil.ReadAll(rawResponse.Body)
	if err != nil {
		return nil, err
	}

	Logger.Debug("DSE-mesos response: %v", string(body))

	errorResponse := make(map[string]interface{})
	err = json.Unmarshal(body, &errorResponse)
	if err == nil {
		if errorMessage, ok := errorResponse["error"].(string); ok {
			if !strings.Contains(errorMessage, "duplicate cluster") {
				return nil, errors.New(errorMessage)
			}
		}
	}

	return body, nil
}

type DSEMesos02xStartResponse struct {
	Status string             `json:"status"`
	Nodes  []*DSEMesos02xNode `json:"nodes"`
}

type DSEMesos02xNode struct {
	Seed    bool                    `json:"seed"`
	State   string                  `json:"state"`
	DC      string                  `json:"dc"`
	ID      string                  `json:"id"`
	Mem     int                     `json:"mem"`
	Cpu     float64                 `json:"mem"`
	Rack    string                  `json:"rack"`
	Runtime *DSEMesos02xNodeRuntime `json:"runtime"`
}

type DSEMesos02xNodeRuntime struct {
	Hostname    string                      `json:"hostname"`
	SlaveID     string                      `json:"slaveId"`
	ExecutorID  string                      `json:"executorId"`
	Attributes  map[string]string           `json:"attributes"`
	Reservation *DSEMesos02xNodeReservation `json:"reservation"`
	Seeds       []string                    `json:"seeds"`
	Address     string                      `json:"address"`
	TaskID      string                      `json:"taskId"`
}

type DSEMesos02xNodeReservation struct {
	Cpus  float64        `json:"cpus"`
	Mem   int            `json:"mem"`
	Ports map[string]int `json:"ports"`
}
