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

type GoKafkaClientTaskRunner struct{}

func (tr *GoKafkaClientTaskRunner) FillContext(context *framework.Context, application *framework.Application, task marathon.Task) error {
	context.Set(fmt.Sprintf("%s.host", application.ID), task.Host)
	for idx, port := range task.Ports {
		context.Set(fmt.Sprintf("%s.port%d", application.ID, idx), fmt.Sprint(port))
	}
	context.Set(fmt.Sprintf("%s.api", application.ID), fmt.Sprintf("http://%s:%d", task.Host, task.Ports[0]))

	return nil
}

func (tr *GoKafkaClientTaskRunner) RunTask(context *framework.Context, application *framework.Application, task map[string]string) error {
	api := context.MustGet(fmt.Sprintf("%s.api", application.ID))

	client := NewGoKafkaClientMesosClient(api)
	err := client.Add(task)
	if err != nil {
		return err
	}

	err = client.Update(task)
	if err != nil {
		return err
	}

	return client.Start(task)
}

type GoKafkaClientMesosClient struct {
	api string
}

func NewGoKafkaClientMesosClient(api string) *GoKafkaClientMesosClient {
	return &GoKafkaClientMesosClient{
		api: api,
	}
}

func (c *GoKafkaClientMesosClient) Add(params map[string]string) error {
	Logger.Info("Adding %s", params["id"])
	values := url.Values{}
	for k, v := range params {
		switch k {
		case "id", "type", "cpu", "mem", "executor":
			{
				values.Set(k, fmt.Sprint(v))
			}
		}
	}

	rawResponse, err := http.Get(fmt.Sprintf("%s/api/add?%s", c.api, values.Encode()))
	if err != nil {
		return err
	}

	return c.checkResponse(rawResponse)
}

func (c *GoKafkaClientMesosClient) Update(params map[string]string) error {
	Logger.Info("Updating %s", params["id"])
	values := url.Values{}
	for k, v := range params {
		switch k {
		case "type", "cpu", "mem", "executor", "timeout":
		default:
			values.Set(k, fmt.Sprint(v))
		}
	}

	rawResponse, err := http.Get(fmt.Sprintf("%s/api/update?%s", c.api, values.Encode()))
	if err != nil {
		return err
	}

	return c.checkResponse(rawResponse)
}

func (c *GoKafkaClientMesosClient) Start(params map[string]string) error {
	Logger.Info("Starting %s", params["id"])
	values := url.Values{}
	for k, v := range params {
		switch k {
		case "id", "timeout":
			values.Set(k, fmt.Sprint(v))
		}
	}

	rawResponse, err := http.Get(fmt.Sprintf("%s/api/start?%s", c.api, values.Encode()))
	if err != nil {
		return err
	}

	return c.checkResponse(rawResponse)
}

func (c *GoKafkaClientMesosClient) checkResponse(rawResponse *http.Response) error {
	body, err := ioutil.ReadAll(rawResponse.Body)
	if err != nil {
		return err
	}

	response := make(map[string]interface{})
	err = json.Unmarshal(body, &response)
	if err != nil {
		return err
	}

	rawSuccess, ok := response["Success"]
	if !ok {
		Logger.Info(string(body))
		return errors.New("Invalid go-kafka-client-mesos response")
	}

	success, ok := rawSuccess.(bool)
	if !ok {
		return errors.New("Invalid go-kafka-client-mesos Success field type")
	}

	Logger.Debug("go-kafka-client-mesos response: %v", response)

	if !success {
		return errors.New(response["Message"].(string))
	}

	return nil
}
