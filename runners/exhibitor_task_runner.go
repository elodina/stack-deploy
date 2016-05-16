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
	"time"
)

type ExhibitorTaskRunner struct{}

func (etr *ExhibitorTaskRunner) FillContext(context *framework.StackContext, application *framework.Application, task marathon.Task) error {
	context.SetStackVariable(fmt.Sprintf("%s.host", application.ID), task.Host)
	for idx, port := range task.Ports {
		context.SetStackVariable(fmt.Sprintf("%s.port%d", application.ID, idx), fmt.Sprint(port))
	}
	context.SetStackVariable(fmt.Sprintf("%s.api", application.ID), fmt.Sprintf("http://%s:%d", task.Host, task.Ports[0]))

	return nil
}

func (etr *ExhibitorTaskRunner) RunTask(context *framework.StackContext, application *framework.Application, task map[string]string) error {
	api := context.MustGet(fmt.Sprintf("%s.api", application.ID))

	client := NewExhibitorMesosClient(api)
	_, err := client.Add(task)
	if err != nil {
		return err
	}

	_, err = client.Update(task)
	if err != nil {
		return err
	}

	response, err := client.Start(task)
	if err != nil {
		return err
	}

	err = etr.fillTaskContext(context, application, response)
	if err != nil {
		return err
	}

	return client.AwaitZookeeperRunning()
}

func (etr *ExhibitorTaskRunner) fillTaskContext(context *framework.StackContext, application *framework.Application, response *exhibitorCluster) error {
	servers := make([]string, 0)
	serversMap := make(map[string]string)

	for _, server := range response.Cluster {
		hostname := fmt.Sprintf("%s:2181", server.Config.Hostname)

		serversMap[server.Id] = hostname
		servers = append(servers, hostname)
	}

	for id, host := range servers {
		context.SetStackVariable(fmt.Sprintf("%s.exhibitor-%s", application.ID, fmt.Sprint(id)), host)
	}

	context.SetStackVariable(fmt.Sprintf("%s.zkConnect", application.ID), strings.Join(servers, ","))
	return nil
}

type ExhibitorMesosClient struct {
	api string
}

func NewExhibitorMesosClient(api string) *ExhibitorMesosClient {
	return &ExhibitorMesosClient{
		api: api,
	}
}

func (emc *ExhibitorMesosClient) Add(params map[string]string) (*exhibitorCluster, error) {
	Logger.Info("Adding %s", params["id"])
	values := url.Values{}
	for k, v := range params {
		switch k {
		case "id", "cpu", "mem", "constraints", "configchangebackoff", "port", "docker":
			values.Set(k, v)
		}
	}

	Logger.Debug(fmt.Sprintf("Requesting %s/api/add?%s", emc.api, values.Encode()))
	rawResponse, err := http.Get(fmt.Sprintf("%s/api/add?%s", emc.api, values.Encode()))
	if err != nil {
		return nil, err
	}

	cluster := new(exhibitorCluster)
	_, err = emc.checkResponse(rawResponse, &cluster)

	return cluster, err
}

func (emc *ExhibitorMesosClient) Update(params map[string]string) (*exhibitorCluster, error) {
	Logger.Info("Updating %s", params["id"])
	values := url.Values{}
	for k, v := range params {
		switch k {
		case "cpu", "mem", "constraints", "configchangebackoff", "port", "docker", "timeout":
		default:
			values.Set(k, v)
		}
	}

	Logger.Debug(fmt.Sprintf("Requesting %s/api/config?%s", emc.api, values.Encode()))
	rawResponse, err := http.Get(fmt.Sprintf("%s/api/config?%s", emc.api, values.Encode()))
	if err != nil {
		return nil, err
	}

	cluster := new(exhibitorCluster)
	_, err = emc.checkResponse(rawResponse, &cluster)

	return cluster, err
}

func (emc *ExhibitorMesosClient) Start(params map[string]string) (*exhibitorCluster, error) {
	Logger.Info("Starting %s", params["id"])
	values := url.Values{}
	for k, v := range params {
		switch k {
		case "id", "timeout":
			values.Set(k, v)
		}
	}

	Logger.Debug(fmt.Sprintf("Requesting %s/api/start?%s", emc.api, values.Encode()))
	rawResponse, err := http.Get(fmt.Sprintf("%s/api/start?%s", emc.api, values.Encode()))
	if err != nil {
		return nil, err
	}

	cluster := new(exhibitorCluster)
	_, err = emc.checkResponse(rawResponse, &cluster)

	return cluster, err
}

func (emc *ExhibitorMesosClient) Status() (*exhibitorClusterStatus, error) {
	Logger.Info("Getting cluster status")

	Logger.Debug(fmt.Sprintf("Requesting %s/api/status", emc.api))
	rawResponse, err := http.Get(fmt.Sprintf("%s/api/status", emc.api))
	if err != nil {
		return nil, err
	}

	clusterStatus := new(exhibitorClusterStatus)
	_, err = emc.checkResponse(rawResponse, &clusterStatus)
	return clusterStatus, err
}

func (emc *ExhibitorMesosClient) AwaitZookeeperRunning() error {
outerLoop:
	for {
		time.Sleep(10 * time.Second)
		status, err := emc.Status()
		if err != nil {
			return err
		}

		targetRunning := len(status.ServerStatuses)
		for _, serverStatus := range status.ServerStatuses {
			actualRunning := 0
			for _, zkStatus := range serverStatus.ExhibitorClusterView {
				if zkStatus.Description == "serving" {
					actualRunning++
				}
			}
			if targetRunning != actualRunning {
				Logger.Info("%d of %d Zookeepers are running, waiting...", actualRunning, targetRunning)
				continue outerLoop
			}
		}

		return nil
	}
}

func (emc *ExhibitorMesosClient) checkResponse(rawResponse *http.Response, unmarshalInto interface{}) (*exhibitorApiResponse, error) {
	body, err := ioutil.ReadAll(rawResponse.Body)
	if err != nil {
		return nil, err
	}

	response := new(exhibitorApiResponse)
	response.Value = unmarshalInto
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	Logger.Debug("Exhibitor-mesos response: %v", string(body))

	if response.Success {
		return response, nil
	}

	return nil, errors.New(response.Message)
}

type exhibitorApiResponse struct {
	Success bool
	Message string
	Value   interface{}
}

type exhibitorCluster struct {
	Frameworkid string
	Cluster     []*exhibitorServer
}

type exhibitorServer struct {
	Id     string
	State  string
	Config *exhibitorServerConfig
}

type exhibitorServerConfig struct {
	ExhibitorConfig      map[string]string
	SharedConfigOverride map[string]string
	Id                   string
	Hostname             string
	Cpu                  float64
	Mem                  float64
	Ports                string
}

type exhibitorClusterStatus struct {
	ServerStatuses []*exhibitorStatus
}

type exhibitorStatus struct {
	Server               *exhibitorServer
	ExhibitorClusterView []*zookeeperStatus
}

type zookeeperStatus struct {
	Hostname    string
	IsLeader    bool
	Description string
	Code        int
}
