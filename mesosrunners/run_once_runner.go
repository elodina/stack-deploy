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

package mesosrunners

import (
	"fmt"
	"github.com/elodina/stack-deploy/framework"
	mesos "github.com/mesos/mesos-go/mesosproto"
	"github.com/mesos/mesos-go/scheduler"
	"strings"
	"sync"
)

type RunOnceRunner struct {
	applications    map[string]*RunOnceApplicationContext
	applicationLock sync.Mutex
}

func NewRunOnceRunner() *RunOnceRunner {
	return &RunOnceRunner{
		applications: make(map[string]*RunOnceApplicationContext),
	}
}

func (r *RunOnceRunner) StageApplication(application *framework.Application, state framework.MesosState) <-chan *framework.ApplicationRunStatus {
	r.applicationLock.Lock()
	defer r.applicationLock.Unlock()

	instances := application.GetInstances(state)
	if instances == 0 {
		statusChan := make(chan *framework.ApplicationRunStatus, 1)
		statusChan <- framework.NewApplicationRunStatus(application, nil)

		return statusChan
	}

	ctx := NewRunOnceApplicationContext()
	ctx.Application = application
	ctx.InstancesLeftToRun = instances
	r.applications[application.ID] = ctx

	return ctx.StatusChan
}

func (r *RunOnceRunner) ResourceOffer(driver scheduler.SchedulerDriver, offer *mesos.Offer) (string, error) {
	r.applicationLock.Lock()
	defer r.applicationLock.Unlock()

	if len(r.applications) == 0 {
		return "all tasks are running", nil
	}

	declineReasons := make([]string, 0)
	for _, ctx := range r.applications {
		declineReason := ctx.Matches(offer)
		if declineReason == "" {
			return "", ctx.LaunchTask(driver, offer)
		}

		declineReasons = append(declineReasons, declineReason)
	}

	return strings.Join(declineReasons, ", "), nil
}

func (r *RunOnceRunner) StatusUpdate(driver scheduler.SchedulerDriver, status *mesos.TaskStatus) bool {
	r.applicationLock.Lock()
	defer r.applicationLock.Unlock()

	applicationID := applicationIDFromTaskID(status.GetTaskId().GetValue())
	ctx, exists := r.applications[applicationID]
	if !exists {
		// this status update was not for run once application, just let it go
		return false
	}

	if ctx.StatusUpdate(driver, status) {
		delete(r.applications, applicationID)
	}

	return true
}

func applicationIDFromTaskID(taskID string) string {
	pipeIndex := strings.Index(taskID, "|")
	if pipeIndex == -1 {
		panic(fmt.Sprintf("Unexpected task ID %s", taskID))
	}

	return taskID[:pipeIndex]
}

func hostnameFromTaskID(taskID string) string {
	pipeIndex := strings.Index(taskID, "|")
	if pipeIndex == -1 {
		panic(fmt.Sprintf("Unexpected task ID %s", taskID))
	}

	hostnameAndUUID := taskID[pipeIndex+1:]
	pipeIndex = strings.Index(hostnameAndUUID, "|")
	if pipeIndex == -1 {
		panic(fmt.Sprintf("Unexpected task ID %s", taskID))
	}

	return hostnameAndUUID[:pipeIndex]
}
