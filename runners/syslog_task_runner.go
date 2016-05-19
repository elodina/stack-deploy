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

type SyslogTaskRunner struct{}

func (str *SyslogTaskRunner) FillContext(context *framework.Variables, application *framework.Application, task marathon.Task) error {
	context.SetStackVariable(fmt.Sprintf("%s.host", application.ID), task.Host)
	for idx, port := range task.Ports {
		context.SetStackVariable(fmt.Sprintf("%s.port%d", application.ID, idx), fmt.Sprint(port))
	}
	context.SetStackVariable(fmt.Sprintf("%s.api", application.ID), fmt.Sprintf("http://%s:%d", task.Host, task.Ports[0]))

	return nil
}

func (str *SyslogTaskRunner) RunTask(context *framework.Variables, application *framework.Application, task map[string]string) error {
	api := context.MustGet(fmt.Sprintf("%s.api", application.ID))

	client := NewSyslogMesosClient(api)
	err := client.Update(task)
	if err != nil {
		return err
	}
	return client.Start(task)
}

type SyslogMesosClient struct {
	api string
}

func NewSyslogMesosClient(api string) *SyslogMesosClient {
	return &SyslogMesosClient{
		api: api,
	}
}

func (smc *SyslogMesosClient) Update(params map[string]string) error {
	Logger.Info("Updating syslog-mesos")
	values := url.Values{}
	for k, v := range params {
		values.Set(k, fmt.Sprint(v))
	}

	rawResponse, err := http.Get(fmt.Sprintf("%s/api/update?%s", smc.api, values.Encode()))
	if err != nil {
		return err
	}

	return smc.checkResponse(rawResponse)
}

func (smc *SyslogMesosClient) Start(params map[string]string) error {
	Logger.Info("Starting syslog-mesos")
	rawResponse, err := http.Get(fmt.Sprintf("%s/api/start", smc.api))
	if err != nil {
		return err
	}

	return smc.checkResponse(rawResponse)
}

func (smc *SyslogMesosClient) checkResponse(rawResponse *http.Response) error {
	if rawResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("Response returned with status code %d", rawResponse.StatusCode)
	}

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
		return errors.New("Invalid syslog-mesos response")
	}

	success, ok := rawSuccess.(bool)
	if !ok {
		return errors.New("Invalid syslog-mesos Success field type")
	}

	Logger.Debug("Syslog-mesos response: %v", response)

	if !success {
		return errors.New(response["Message"].(string))
	}

	return nil
}
