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
)

type ZipkinTaskRunner struct{}

func (ztr *ZipkinTaskRunner) FillContext(context *framework.Variables, application *framework.Application, task marathon.Task) error {
	context.SetStackVariable(fmt.Sprintf("%s.host", application.ID), task.Host)
	for idx, port := range task.Ports {
		context.SetStackVariable(fmt.Sprintf("%s.port%d", application.ID, idx), fmt.Sprint(port))
	}
	context.SetStackVariable(fmt.Sprintf("%s.api", application.ID), fmt.Sprintf("http://%s:%d", task.Host, task.Ports[0]))

	return nil
}

func (ztr *ZipkinTaskRunner) RunTask(context *framework.Variables, application *framework.Application, task map[string]string) error {
	api := context.MustGet(fmt.Sprintf("%s.api", application.ID))

	id, ok := task["id"]
	if !ok {
		return errors.New("Missing task id")
	}

	taskType, ok := task["type"]
	if !ok {
		return fmt.Errorf("Missing task type for id %s", id)
	}

	client := NewZipkinMesosClient(api)
	_, err := client.Add(task)
	if err != nil {
		return err
	}

	response, err := client.Start(task)
	if err != nil {
		return err
	}

	if taskType == "query" {
		return ztr.fillTaskContext(context, application, response)
	}

	return nil
}

func (ztr *ZipkinTaskRunner) fillTaskContext(context *framework.Variables, application *framework.Application, response map[string]interface{}) error {
	valueArr, ok := response["value"].([]interface{})
	if !ok {
		return errors.New("Wrong value field")
	}

	for _, valueIface := range valueArr {
		value, ok := valueIface.(map[string]interface{})
		if !ok {
			return errors.New("Wrong value interface field")
		}

		id, ok := value["id"].(string)
		if !ok {
			return errors.New("Wrong id field")
		}

		config, ok := value["config"].(map[string]interface{})
		if !ok {
			return errors.New("Wrong config field")
		}

		hostname, ok := config["hostname"].(string)
		if !ok {
			return errors.New("Wrong hostname field")
		}

		env, ok := config["env"].(map[string]interface{})
		if !ok {
			return errors.New("Wrong env field")
		}

		queryPort, ok := env["QUERY_PORT"].(string)
		if !ok {
			return errors.New("Wrong QUERY_PORT env")
		}

		context.SetStackVariable(fmt.Sprintf("%s.query-%s.endpoint", application.ID, id), fmt.Sprintf("%s:%s", hostname, queryPort))
	}

	return nil
}

type ZipkinMesosClient struct {
	api string
}

func NewZipkinMesosClient(api string) *ZipkinMesosClient {
	return &ZipkinMesosClient{
		api: api,
	}
}

func (zmc *ZipkinMesosClient) Add(params map[string]string) (map[string]interface{}, error) {
	Logger.Info("Adding %s", params["id"])

	taskType := params["type"]

	values := url.Values{}
	for k, v := range params {
		switch k {
		case "id", "cpu", "mem", "flags", "env", "port", "adminPort", "configFile", "constraints":
			values.Set(k, v)
		}
	}

	Logger.Debug(fmt.Sprintf("Requesting %s/api/%s/add?%s", zmc.api, taskType, values.Encode()))
	rawResponse, err := http.Get(fmt.Sprintf("%s/api/%s/add?%s", zmc.api, taskType, values.Encode()))
	if err != nil {
		return nil, err
	}

	return zmc.checkResponse(rawResponse)
}

func (zmc *ZipkinMesosClient) Start(params map[string]string) (map[string]interface{}, error) {
	Logger.Info("Starting %s", params["id"])

	taskType := params["type"]

	values := url.Values{}
	for k, v := range params {
		switch k {
		case "id", "timeout":
			values.Set(k, v)
		}
	}

	Logger.Debug(fmt.Sprintf("Requesting %s/api/%s/start?%s", zmc.api, taskType, values.Encode()))
	rawResponse, err := http.Get(fmt.Sprintf("%s/api/%s/start?%s", zmc.api, taskType, values.Encode()))
	if err != nil {
		return nil, err
	}

	return zmc.checkResponse(rawResponse)
}

func (zmc *ZipkinMesosClient) checkResponse(rawResponse *http.Response) (map[string]interface{}, error) {
	body, err := ioutil.ReadAll(rawResponse.Body)
	if err != nil {
		return nil, err
	}

	response := make(map[string]interface{})
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	Logger.Debug("Zipkin-mesos response: %v", response)

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
