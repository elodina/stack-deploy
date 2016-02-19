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
	utils "github.com/elodina/go-mesos-utils"
	"github.com/elodina/go-mesos-utils/pretty"
	"github.com/elodina/stack-deploy/framework"
	"github.com/golang/protobuf/proto"
	mesos "github.com/mesos/mesos-go/mesosproto"
	util "github.com/mesos/mesos-go/mesosutil"
	"github.com/mesos/mesos-go/scheduler"
	"sync"
)

type RunOnceApplicationContext struct {
	Application *framework.Application
	StatusChan  chan *framework.ApplicationRunStatus

	InstancesLeftToRun int

	lock            sync.RWMutex
	stagedInstances map[string]mesos.TaskState
}

func NewRunOnceApplicationContext() *RunOnceApplicationContext {
	return &RunOnceApplicationContext{
		stagedInstances: make(map[string]mesos.TaskState),
	}
}

func (ctx *RunOnceApplicationContext) Matches(offer *mesos.Offer) string {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	if ctx.InstancesLeftToRun == 0 {
		return "all instances are staged/running"
	}

	if _, exists := ctx.stagedInstances[offer.GetHostname()]; exists {
		return fmt.Sprintf("application instance is already staged/running on host %s", offer.GetHostname())
	}

	if ctx.Application.Cpu > utils.GetScalarResources(offer, utils.ResourceCpus) {
		return "no cpus"
	}

	if ctx.Application.Mem > utils.GetScalarResources(offer, utils.ResourceMem) {
		return "no mem"
	}

	return ""
}

func (ctx *RunOnceApplicationContext) LaunchTask(driver scheduler.SchedulerDriver, offer *mesos.Offer) error {
	ctx.lock.Lock()
	defer ctx.lock.Unlock()

	ctx.InstancesLeftToRun--
	ctx.stagedInstances[offer.GetHostname()] = mesos.TaskState_TASK_STAGING
	taskInfo := ctx.newTaskInfo(offer)

	_, err := driver.LaunchTasks([]*mesos.OfferID{offer.GetId()}, []*mesos.TaskInfo{taskInfo}, &mesos.Filters{RefuseSeconds: proto.Float64(10)})
	return err
}

func (ctx *RunOnceApplicationContext) StatusUpdate(driver scheduler.SchedulerDriver, status *mesos.TaskStatus) bool {
	ctx.lock.Lock()
	defer ctx.lock.Unlock()

	hostname := hostnameFromTaskID(status.GetTaskId().GetValue())

	ctx.stagedInstances[hostname] = status.GetState()

	switch status.GetState() {
	case mesos.TaskState_TASK_RUNNING:
		framework.Logger.Debug("Task %s received status update in state %s", status.GetTaskId().GetValue(), status.GetState().String())
	case mesos.TaskState_TASK_LOST, mesos.TaskState_TASK_FAILED, mesos.TaskState_TASK_ERROR:
		//TODO also kill all other running tasks sometime?
		ctx.StatusChan <- framework.NewApplicationRunStatus(ctx.Application, fmt.Errorf("Application %s failed to run on host %s with status %s: %s", ctx.Application.ID, hostname, status.GetState().String(), status.GetMessage()))
		return true
	case mesos.TaskState_TASK_FINISHED, mesos.TaskState_TASK_KILLED:
		if ctx.allTasksFinished() {
			ctx.StatusChan <- framework.NewApplicationRunStatus(ctx.Application, nil)
			return true
		}
	default:
		framework.Logger.Warn("Got unexpected task state %s", pretty.Status(status))
	}

	return false
}

func (ctx *RunOnceApplicationContext) newTaskInfo(offer *mesos.Offer) *mesos.TaskInfo {
	taskName := fmt.Sprintf("%s.%s", ctx.Application.ID, offer.GetHostname())
	taskID := util.NewTaskID(fmt.Sprintf("%s|%s|%s", ctx.Application.ID, offer.GetHostname(), framework.UUID()))

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

func (ctx *RunOnceApplicationContext) allTasksFinished() bool {
	framework.Logger.Debug("Checking if all tasks finished for application %s", ctx.Application.ID)

	if ctx.InstancesLeftToRun != 0 {
		framework.Logger.Debug("%d tasks for application %s not yet staged", ctx.InstancesLeftToRun, ctx.Application.ID)
		return false
	}

	for hostname, state := range ctx.stagedInstances {
		if state != mesos.TaskState_TASK_FINISHED && state != mesos.TaskState_TASK_KILLED {
			framework.Logger.Debug("Task on hostname %s for application %s is not yet finished/killed", hostname, ctx.Application.ID)
			return false
		}
	}

	return true
}
