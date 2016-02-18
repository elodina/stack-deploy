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
	"fmt"
	"github.com/elodina/go-mesos-utils/pretty"
	"github.com/golang/protobuf/proto"
	mesos "github.com/mesos/mesos-go/mesosproto"
	util "github.com/mesos/mesos-go/mesosutil"
	"github.com/mesos/mesos-go/scheduler"
	"strings"
	"sync"
)

type MesosApplicationContext struct {
	Application *Application
	State       ApplicationState
	StatusChan  chan *ApplicationRunStatus

	InstancesLeftToRun int

	lock            sync.RWMutex
	stagedInstances map[string]mesos.TaskState
}

func NewMesosApplicationContext() *MesosApplicationContext {
	return &MesosApplicationContext{
		stagedInstances: make(map[string]mesos.TaskState),
	}
}

func (ctx *MesosApplicationContext) Matches(offer *mesos.Offer) string {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	if ctx.InstancesLeftToRun == 0 {
		return "all instances are staged/running"
	}

	if _, exists := ctx.stagedInstances[offer.GetHostname()]; exists {
		return fmt.Sprintf("application instance is already staged/running on this host")
	}

	if ctx.Application.Cpu > getScalarResources(offer, "cpus") {
		return "no cpus"
	}

	if ctx.Application.Mem > getScalarResources(offer, "mem") {
		return "no mem"
	}

	return ""
}

func (ctx *MesosApplicationContext) LaunchTask(driver scheduler.SchedulerDriver, offer *mesos.Offer) {
	ctx.lock.Lock()
	defer ctx.lock.Unlock()

	ctx.InstancesLeftToRun--
	ctx.stagedInstances[offer.GetHostname()] = mesos.TaskState_TASK_STAGING
	taskInfo := ctx.newTaskInfo(offer)

	driver.LaunchTasks([]*mesos.OfferID{offer.GetId()}, []*mesos.TaskInfo{taskInfo}, &mesos.Filters{RefuseSeconds: proto.Float64(10)})
}

func (ctx *MesosApplicationContext) StatusUpdate(driver scheduler.SchedulerDriver, status *mesos.TaskStatus) bool {
	ctx.lock.Lock()
	defer ctx.lock.Unlock()

	hostname := hostnameFromTaskID(status.GetTaskId().GetValue())

	ctx.stagedInstances[hostname] = status.GetState()

	switch status.GetState() {
	case mesos.TaskState_TASK_RUNNING:
		Logger.Debug("Task %s received status update in state %s", status.GetTaskId().GetValue(), status.GetState().String())
	case mesos.TaskState_TASK_LOST, mesos.TaskState_TASK_FAILED, mesos.TaskState_TASK_ERROR:
		//TODO also kill all other running tasks
		ctx.StatusChan <- NewApplicationRunStatus(ctx.Application, fmt.Errorf("Application %s failed to run on host %s with status %s: %s", ctx.Application.ID, hostname, status.GetState().String(), status.GetMessage()))
		return true
	case mesos.TaskState_TASK_FINISHED, mesos.TaskState_TASK_KILLED:
		if ctx.allTasksFinished() {
			ctx.StatusChan <- NewApplicationRunStatus(ctx.Application, nil)
			return true
		}
	default:
		Logger.Warn("Got unexpected task state %s", pretty.Status(status))
	}

	return false
}

func (ctx *MesosApplicationContext) newTaskInfo(offer *mesos.Offer) *mesos.TaskInfo {
	taskName := fmt.Sprintf("%s.%s", ctx.Application.ID, offer.GetHostname())
	taskID := util.NewTaskID(fmt.Sprintf("%s|%s|%s", ctx.Application.ID, offer.GetHostname(), UUID()))

	return &mesos.TaskInfo{
		Name:    proto.String(taskName),
		TaskId:  taskID,
		SlaveId: offer.GetSlaveId(),
		Resources: []*mesos.Resource{
			util.NewScalarResource("cpus", ctx.Application.Cpu),
			util.NewScalarResource("mem", ctx.Application.Mem),
		},
		Command: &mesos.CommandInfo{
			Shell: proto.Bool(true),
			Value: proto.String(ctx.Application.LaunchCommand),
		},
	}
}

func (ctx *MesosApplicationContext) allTasksFinished() bool {
	Logger.Debug("Checking if all tasks finished for application %s", ctx.Application.ID)

	if ctx.InstancesLeftToRun != 0 {
		Logger.Debug("%d tasks for application %s not yet staged", ctx.InstancesLeftToRun, ctx.Application.ID)
		return false
	}

	for hostname, state := range ctx.stagedInstances {
		if state != mesos.TaskState_TASK_FINISHED && state != mesos.TaskState_TASK_KILLED {
			Logger.Debug("Task on hostname %s for application %s is not yet finished/killed", hostname, ctx.Application.ID)
			return false
		}
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
