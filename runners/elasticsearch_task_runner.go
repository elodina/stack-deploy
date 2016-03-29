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
	mesos "github.com/mesos/mesos-go/mesosproto"
	"io/ioutil"
	"net/http"
	"time"
)

type ElasticsearchTaskRunner struct {
	awaitTimeout   time.Duration
	backoffTimeout time.Duration
}

func NewElasticsearchTaskRunner(awaitTimeout time.Duration, backoffTimeout time.Duration) *ElasticsearchTaskRunner {
	return &ElasticsearchTaskRunner{
		awaitTimeout:   awaitTimeout,
		backoffTimeout: backoffTimeout,
	}
}

func (etr *ElasticsearchTaskRunner) FillContext(context *framework.StackContext, application *framework.Application, task marathon.Task) error {
	context.SetStackVariable(fmt.Sprintf("%s.host", application.ID), task.Host)
	for idx, port := range task.Ports {
		context.SetStackVariable(fmt.Sprintf("%s.port%d", application.ID, idx), fmt.Sprint(port))
	}
	api := fmt.Sprintf("http://%s:%d", task.Host, task.Ports[0])
	context.SetStackVariable(fmt.Sprintf("%s.api", application.ID), api)

	client := NewElasticsearchClient(api)
	err := client.AwaitRunning(etr.awaitTimeout, etr.backoffTimeout)
	if err != nil {
		return err
	}

	tasks, err := client.GetTasks()
	if err != nil {
		return err
	}

	for idx, task := range tasks {
		context.SetStackVariable(fmt.Sprintf("%s.%d.httpAddress", application.ID, idx), task.HttpAddress)
		context.SetStackVariable(fmt.Sprintf("%s.%d.transportAddress", application.ID, idx), task.TransportAddress)
	}

	return nil
}

func (etr *ElasticsearchTaskRunner) RunTask(context *framework.StackContext, application *framework.Application, task map[string]string) error {
	return errors.New("Elasticsearch task runner does not support running tasks.")
}

type ElasticsearchClient struct {
	api string
}

func NewElasticsearchClient(api string) *ElasticsearchClient {
	return &ElasticsearchClient{
		api: api,
	}
}

func (c *ElasticsearchClient) AwaitRunning(awaitTimeout time.Duration, backoffTimeout time.Duration) error {
	tick := time.NewTicker(awaitTimeout)
outerLoop:
	for {
		select {
		case <-tick.C:
			tick.Stop()
			return fmt.Errorf("Failed to await for running Elasticsearch tasks after %s", awaitTimeout)
		default:
			tasks, err := c.GetTasks()
			if err != nil {
				Logger.Warn("Error fetching Elasticsearch tasks: %s", err)
				time.Sleep(backoffTimeout)
				continue
			}

			if len(tasks) == 0 {
				Logger.Debug("Elasticsearch scheduler returned empty tasks list: %s")
				time.Sleep(backoffTimeout)
				continue
			}

			for _, task := range tasks {
				if task.State != mesos.TaskState_TASK_RUNNING.String() {
					Logger.Debug("Elasticsearch task %d is in state %s, waiting...", task.ID, task.State)
					time.Sleep(backoffTimeout)
					continue outerLoop
				}
			}

			return nil
		}
	}
}

func (c *ElasticsearchClient) GetTasks() ([]*ElasticsearchTask, error) {
	rawResponse, err := http.Get(fmt.Sprintf("%s/v1/tasks", c.api))
	if err != nil {
		return nil, err
	}
	defer rawResponse.Body.Close()

	responseJson, err := ioutil.ReadAll(rawResponse.Body)
	if err != nil {
		return nil, err
	}

	var tasks []*ElasticsearchTask
	err = json.Unmarshal(responseJson, &tasks)
	if err != nil {
		return nil, err
	}

	return tasks, nil
}

type ElasticsearchTask struct {
	ID               string `json:"id"`
	State            string `json:"state"`
	Name             string `json:"name"`
	Version          string `json:"version"`
	StartedAt        string `json:"started_at"`
	HttpAddress      string `json:"http_address"`
	TransportAddress string `json:"transport_address"`
	Hostname         string `json:"hostname"`
}
