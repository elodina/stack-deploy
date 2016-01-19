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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/elodina/stack-deploy/framework"
	"github.com/gambol99/go-marathon"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

type DSETaskRunner struct{}

func (dtr *DSETaskRunner) FillContext(context *framework.Context, application *framework.Application, task marathon.Task) error {
	context.Set(fmt.Sprintf("%s.host", application.ID), task.Host)
	for idx, port := range task.Ports {
		context.Set(fmt.Sprintf("%s.port%d", application.ID, idx), fmt.Sprint(port))
	}
	context.Set(fmt.Sprintf("%s.api", application.ID), fmt.Sprintf("http://%s:%d", task.Host, task.Ports[0]))

	return nil
}

func (dtr *DSETaskRunner) RunTask(context *framework.Context, application *framework.Application, task map[string]string) error {
	api := context.MustGet(fmt.Sprintf("%s.api", application.ID))

	client := NewDSEMesosClient(api)
	_, err := client.Add(task)
	if err != nil {
		return err
	}

	response, err := client.Start(task)
	if err != nil {
		return err
	}

	return dtr.fillTaskContext(context, application, response)
}

func (dtr *DSETaskRunner) fillTaskContext(context *framework.Context, application *framework.Application, response map[string]interface{}) error {
	servers := make([]string, 0)
	serversMap := make(map[string]string)

	value, ok := response["value"].(map[string]interface{})
	if !ok {
		return errors.New("Wrong value field")
	}

	clusterArr, ok := value["cluster"].([]interface{})
	if !ok {
		return errors.New("Wrong cluster field")
	}

	for _, clusterIface := range clusterArr {
		cluster, ok := clusterIface.(map[string]interface{})
		if !ok {
			return errors.New("Wrong cluster field")
		}

		id, ok := cluster["id"].(string)
		if !ok {
			return errors.New("Wrong id field")
		}

		runtime, ok := cluster["runtime"].(map[string]interface{})
		if !ok {
			return errors.New("Wrong runtime field")
		}

		hostname, ok := runtime["hostname"].(string)
		if !ok {
			return errors.New("Wrong hostname field")
		}

		serversMap[id] = hostname
		servers = append(servers, hostname)
	}

	for id, host := range servers {
		context.Set(fmt.Sprintf("%s.cassandra-node-%s", application.ID, fmt.Sprint(id)), host)
	}

	context.Set(fmt.Sprintf("%s.cassandraConnect", application.ID), strings.Join(servers, ","))
	return nil
}

type DSEMesosClient struct {
	api string
}

func NewDSEMesosClient(api string) *DSEMesosClient {
	return &DSEMesosClient{
		api: api,
	}
}

func (dmc *DSEMesosClient) Add(params map[string]string) (map[string]interface{}, error) {
	Logger.Info("Adding %s", params["id"])

	return dmc.doPost(fmt.Sprintf("%s/api/add", dmc.api), params)
}

func (dmc *DSEMesosClient) Start(params map[string]string) (map[string]interface{}, error) {
	Logger.Info("Starting %s", params["id"])

	return dmc.doPost(fmt.Sprintf("%s/api/start", dmc.api), params)
}

func (dmc *DSEMesosClient) doPost(url string, params map[string]string) (map[string]interface{}, error) {
	postParams := make(map[string]interface{})
	for k, v := range params {
		switch k {
		case "cpu", "mem":
			{
				floatV, err := strconv.ParseFloat(v, 64)
				if err != nil {
					return nil, err
				}
				postParams[k] = floatV
			}
		case "seed":
			{
				boolV, err := strconv.ParseBool(v)
				if err != nil {
					return nil, err
				}
				postParams[k] = boolV
			}
		default:
			postParams[k] = v
		}
	}

	body, err := json.Marshal(postParams)
	if err != nil {
		return nil, err
	}

	rawResponse, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	return dmc.checkResponse(rawResponse)
}

func (dmc *DSEMesosClient) checkResponse(rawResponse *http.Response) (map[string]interface{}, error) {
	body, err := ioutil.ReadAll(rawResponse.Body)
	if err != nil {
		return nil, err
	}

	response := make(map[string]interface{})
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	Logger.Debug("DSE-mesos response: %v", response)

	success, ok := response["success"].(bool)
	if ok && success {
		return response, nil
	}

	message, ok := response["message"].(string)
	if ok {
		return nil, errors.New(message)
	}

	return nil, errors.New("Request failed for unknown reason")
}
