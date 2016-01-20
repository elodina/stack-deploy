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
	"github.com/gambol99/go-marathon"
	"net/url"
	"time"
)

type MockMarathon struct {
	applications map[string]*marathon.Application
	tasks        map[string]*marathon.Tasks
	err          error
}

func NewMockMarathon() *MockMarathon {
	return &MockMarathon{
		applications: make(map[string]*marathon.Application),
		tasks:        make(map[string]*marathon.Tasks),
	}
}

func (m *MockMarathon) AbdicateLeader() (string, error) {
	return "", nil
}

func (m *MockMarathon) AddEventsListener(channel marathon.EventsChannel, filter int) error {
	return nil
}

func (m *MockMarathon) AllTasks(opts *marathon.AllTasksOpts) (*marathon.Tasks, error) {
	return nil, nil
}

func (m *MockMarathon) Application(name string) (*marathon.Application, error) {
	return m.applications[name], m.err
}

func (m *MockMarathon) ApplicationDeployments(name string) ([]*marathon.DeploymentID, error) {
	return nil, nil
}

func (m *MockMarathon) ApplicationOK(name string) (bool, error) {
	return false, nil
}

func (m *MockMarathon) Applications(url.Values) (*marathon.Applications, error) {
	return nil, nil
}

func (m *MockMarathon) ApplicationVersions(name string) (*marathon.ApplicationVersions, error) {
	return nil, nil
}

func (m *MockMarathon) CreateApplication(application *marathon.Application) (*marathon.Application, error) {
	return nil, nil
}

func (m *MockMarathon) CreateGroup(group *marathon.Group) error {
	return nil
}

func (m *MockMarathon) DeleteApplication(name string) (*marathon.DeploymentID, error) {
	return nil, nil
}

func (m *MockMarathon) DeleteDeployment(id string, force bool) (*marathon.DeploymentID, error) {
	return nil, nil
}

func (m *MockMarathon) DeleteGroup(name string) (*marathon.DeploymentID, error) {
	return nil, nil
}

func (m *MockMarathon) Deployments() ([]*marathon.Deployment, error) {
	return nil, nil
}

func (m *MockMarathon) GetMarathonURL() string {
	return ""
}

func (m *MockMarathon) Group(name string) (*marathon.Group, error) {
	return nil, nil
}

func (m *MockMarathon) Groups() (*marathon.Groups, error) {
	return nil, nil
}

func (m *MockMarathon) HasApplicationVersion(name, version string) (bool, error) {
	return false, nil
}

func (m *MockMarathon) HasDeployment(id string) (bool, error) {
	return false, nil
}

func (m *MockMarathon) HasGroup(name string) (bool, error) {
	return false, nil
}

func (m *MockMarathon) Info() (*marathon.Info, error) {
	return nil, nil
}

func (m *MockMarathon) KillApplicationTasks(applicationID string, opts *marathon.KillApplicationTasksOpts) (*marathon.Tasks, error) {
	return nil, nil
}

func (m *MockMarathon) KillTask(taskID string, opts *marathon.KillTaskOpts) (*marathon.Task, error) {
	return nil, nil
}

func (m *MockMarathon) KillTasks(taskIDs []string, opts *marathon.KillTaskOpts) error {
	return nil
}

func (m *MockMarathon) Leader() (string, error) {
	return "", nil
}

func (m *MockMarathon) Ping() (bool, error) {
	return false, nil
}

func (m *MockMarathon) RemoveEventsListener(channel marathon.EventsChannel) {

}

func (m *MockMarathon) RestartApplication(name string, force bool) (*marathon.DeploymentID, error) {
	return nil, nil
}

func (m *MockMarathon) ScaleApplicationInstances(name string, instances int, force bool) (*marathon.DeploymentID, error) {
	return nil, nil
}

func (m *MockMarathon) SetApplicationVersion(name string, version *marathon.ApplicationVersion) (*marathon.DeploymentID, error) {
	return nil, nil
}

func (m *MockMarathon) Subscriptions() (*marathon.Subscriptions, error) {
	return nil, nil
}

func (m *MockMarathon) TaskEndpoints(name string, port int, healthCheck bool) ([]string, error) {
	return nil, nil
}

func (m *MockMarathon) Tasks(application string) (*marathon.Tasks, error) {
	return m.tasks[application], m.err
}

func (m *MockMarathon) Unsubscribe(string) error {
	return nil
}

func (m *MockMarathon) UpdateApplication(application *marathon.Application) (*marathon.DeploymentID, error) {
	return nil, nil
}

func (m *MockMarathon) UpdateGroup(id string, group *marathon.Group) (*marathon.DeploymentID, error) {
	return nil, nil
}

func (m *MockMarathon) WaitOnApplication(name string, timeout time.Duration) error {
	return nil
}

func (m *MockMarathon) WaitOnDeployment(id string, timeout time.Duration) error {
	return nil
}

func (m *MockMarathon) WaitOnGroup(name string, timeout time.Duration) error {
	return nil
}

func (m *MockMarathon) ListApplications(url.Values) ([]string, error) {
	return nil, nil
}
