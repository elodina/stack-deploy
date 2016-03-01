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
	"github.com/elodina/stack-deploy/constraints"
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

	lock  sync.RWMutex
	tasks []*runOnceTask
}

func NewRunOnceApplicationContext() *RunOnceApplicationContext {
	tasks := make([]*runOnceTask, 0)

	return &RunOnceApplicationContext{
		StatusChan: make(chan *framework.ApplicationRunStatus),
		tasks:      tasks,
	}
}

func (ctx *RunOnceApplicationContext) Matches(offer *mesos.Offer) string {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	if ctx.InstancesLeftToRun == 0 {
		return "all instances are staged/running"
	}

	declineReason := ctx.CheckConstraints(offer)
	if declineReason != "" {
		return declineReason
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
	taskInfo := ctx.newTaskInfo(offer)
	ctx.tasks = append(ctx.tasks, newRunOnceTask(offer, taskInfo.GetTaskId().GetValue()))

	_, err := driver.LaunchTasks([]*mesos.OfferID{offer.GetId()}, []*mesos.TaskInfo{taskInfo}, &mesos.Filters{RefuseSeconds: proto.Float64(10)})
	return err
}

func (ctx *RunOnceApplicationContext) StatusUpdate(driver scheduler.SchedulerDriver, status *mesos.TaskStatus) bool {
	ctx.lock.Lock()
	defer ctx.lock.Unlock()

	hostname := hostnameFromTaskID(status.GetTaskId().GetValue())
	ctx.updateTaskState(status)

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

	var URIs []*mesos.CommandInfo_URI
	if len(ctx.Application.ArtifactURLs) > 0 || len(ctx.Application.AdditionalArtifacts) > 0 {
		URIs = make([]*mesos.CommandInfo_URI, 0)
		for _, uri := range ctx.Application.ArtifactURLs {
			URIs = append(URIs, &mesos.CommandInfo_URI{
				Value:   proto.String(uri),
				Extract: proto.Bool(true),
			})
		}
		for _, uri := range ctx.Application.AdditionalArtifacts {
			URIs = append(URIs, &mesos.CommandInfo_URI{
				Value:   proto.String(uri),
				Extract: proto.Bool(true),
			})
		}
	}

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
			Uris:  URIs,
		},
	}
}

func (ctx *RunOnceApplicationContext) allTasksFinished() bool {
	framework.Logger.Debug("Checking if all tasks finished for application %s", ctx.Application.ID)

	if ctx.InstancesLeftToRun != 0 {
		framework.Logger.Debug("%d tasks for application %s not yet staged", ctx.InstancesLeftToRun, ctx.Application.ID)
		return false
	}

	for _, task := range ctx.tasks {
		if task.State != mesos.TaskState_TASK_FINISHED && task.State != mesos.TaskState_TASK_KILLED {
			framework.Logger.Debug("Task with id %s for application %s is not yet finished/killed", task.TaskID, ctx.Application.ID)
			return false
		}
	}

	return true
}

func (ctx *RunOnceApplicationContext) updateTaskState(status *mesos.TaskStatus) {
	for _, task := range ctx.tasks {
		if task.TaskID == status.GetTaskId().GetValue() {
			task.State = status.GetState()
			return
		}
	}

	framework.Logger.Warn("Got unexpected status update for unknown task with ID %s", status.GetTaskId().GetValue())
}

func (ctx *RunOnceApplicationContext) CheckConstraints(offer *mesos.Offer) string {
	offerAttributes := constraints.OfferAttributes(offer)

	for name, constraints := range ctx.Application.GetConstraints() {
		for _, constraint := range constraints {
			attribute, exists := offerAttributes[name]
			if exists {
				if !constraint.Matches(attribute, ctx.otherTasksAttributes(name)) {
					framework.Logger.Debug("Attribute %s doesn't match %s", name, constraint)
					return fmt.Sprintf("%s doesn't match %s", name, constraint)
				}
			} else {
				framework.Logger.Debug("Offer does not contain %s attribute", name)
				return fmt.Sprintf("no %s", name)
			}
		}
	}

	return ""
}

func (ctx *RunOnceApplicationContext) otherTasksAttributes(name string) []string {
	attributes := make([]string, 0)
	for _, task := range ctx.tasks {
		if task.State != mesos.TaskState_TASK_STARTING {
			value := task.Attributes[name]
			if value != "" {
				attributes = append(attributes, value)
			}
		}
	}

	return attributes
}

type runOnceTask struct {
	State      mesos.TaskState
	Attributes map[string]string
	TaskID     string
}

func newRunOnceTask(offer *mesos.Offer, taskID string) *runOnceTask {
	return &runOnceTask{
		State:      mesos.TaskState_TASK_STAGING,
		Attributes: constraints.OfferAttributes(offer),
		TaskID:     taskID,
	}
}
