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
	mesos "github.com/mesos/mesos-go/mesosproto"
	"github.com/mesos/mesos-go/scheduler"
	"github.com/yanzay/cron"
)

type CronScheduler interface {
	AddFunc(string, func()) (int64, error)
	DeleteJob(int64)
	Entries() []*cron.Entry
}

var TaskRunners map[string]TaskRunner
var MesosTaskRunners map[string]MesosTaskRunner

type TaskRunner interface {
	FillContext(context *StackContext, application *Application, task marathon.Task) error
	RunTask(context *StackContext, application *Application, task map[string]string) error
}

type MesosTaskRunner interface {
	ScheduleApplication(*Application, MesosState, CronScheduler) (int64, <-chan *ApplicationRunStatus)
	DeleteSchedule(int64, CronScheduler)
	StageApplication(application *Application, mesos MesosState) <-chan *ApplicationRunStatus
	ResourceOffer(driver scheduler.SchedulerDriver, offer *mesos.Offer) (string, error)
	StatusUpdate(driver scheduler.SchedulerDriver, status *mesos.TaskStatus) bool
}
