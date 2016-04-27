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

package utils

import (
	mesos "github.com/mesos/mesos-go/mesosproto"
	util "github.com/mesos/mesos-go/mesosutil"
	"github.com/mesos/mesos-go/scheduler"
	"sync"
	"time"
)

type Reconciler struct {
	ReconcileDelay    time.Duration
	ReconcileMaxTries int

	tasks         map[string]struct{}
	taskLock      sync.Mutex
	reconcileTime time.Time
	reconciles    int
}

func NewReconciler() *Reconciler {
	return &Reconciler{
		ReconcileDelay:    10 * time.Second,
		ReconcileMaxTries: 3,
		tasks:             make(map[string]struct{}),
		reconcileTime:     time.Unix(0, 0),
	}
}

func (r *Reconciler) ImplicitReconcile(driver scheduler.SchedulerDriver) {
	r.reconcile(driver, true)
}

func (r *Reconciler) ExplicitReconcile(taskIDs []string, driver scheduler.SchedulerDriver) {
	r.taskLock.Lock()
	for _, taskID := range taskIDs {
		r.tasks[taskID] = struct{}{}
	}
	r.taskLock.Unlock()

	r.reconcile(driver, false)
}

func (r *Reconciler) Update(status *mesos.TaskStatus) {
	r.taskLock.Lock()
	defer r.taskLock.Unlock()

	delete(r.tasks, status.GetTaskId().GetValue())

	if len(r.tasks) == 0 {
		r.reconciles = 0
	}
}

func (r *Reconciler) reconcile(driver scheduler.SchedulerDriver, implicit bool) {
	if time.Now().Sub(r.reconcileTime) >= r.ReconcileDelay {
		r.taskLock.Lock()
		defer r.taskLock.Unlock()

		r.reconciles++
		r.reconcileTime = time.Now()

		if r.reconciles > r.ReconcileMaxTries {
			for task := range r.tasks {
				Logger.Info("Reconciling exceeded %d tries, sending killTask for task %s", r.ReconcileMaxTries, task)
				driver.KillTask(util.NewTaskID(task))
			}
			r.reconciles = 0
		} else {
			if implicit {
				driver.ReconcileTasks(nil)
			} else {
				statuses := make([]*mesos.TaskStatus, 0)
				for task := range r.tasks {
					Logger.Debug("Reconciling %d/%d task state for task id %s", r.reconciles, r.ReconcileMaxTries, task)
					statuses = append(statuses, util.NewTaskStatus(util.NewTaskID(task), mesos.TaskState_TASK_STAGING))
				}
				driver.ReconcileTasks(statuses)
			}
		}
	}
}
