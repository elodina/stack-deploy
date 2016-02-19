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

package testing

import (
	mesos "github.com/mesos/mesos-go/mesosproto"
)

type MockSchedulerDriver struct {
	StartStatus mesos.Status
	StartError  error
	StartCount  int

	StopStatus mesos.Status
	StopError  error
	StopCount  int

	AbortStatus mesos.Status
	AbortError  error
	AbortCount  int

	JoinStatus mesos.Status
	JoinError  error
	JoinCount  int

	RunStatus mesos.Status
	RunError  error
	RunCount  int

	RequestResourcesStatus mesos.Status
	RequestResourcesError  error
	RequestResourcesCount  int

	LaunchTasksStatus mesos.Status
	LaunchTasksError  error
	LaunchTasksCount  int

	KillTaskStatus mesos.Status
	KillTaskError  error
	KillTaskCount  int

	DeclineOfferStatus mesos.Status
	DeclineOfferError  error
	DeclineOfferCount  int

	ReviveOffersStatus mesos.Status
	ReviveOffersError  error
	ReviveOffersCount  int

	SendFrameworkMessageStatus mesos.Status
	SendFrameworkMessageError  error
	SendFrameworkMessageCount  int

	ReconcileTasksStatus mesos.Status
	ReconcileTasksError  error
	ReconcileTasksCount  int
}

func (s *MockSchedulerDriver) Start() (mesos.Status, error) {
	s.StartCount++
	return s.StartStatus, s.StartError
}

func (s *MockSchedulerDriver) Stop(failover bool) (mesos.Status, error) {
	s.StopCount++
	return s.StopStatus, s.StopError
}

func (s *MockSchedulerDriver) Abort() (mesos.Status, error) {
	s.AbortCount++
	return s.AbortStatus, s.AbortError
}

func (s *MockSchedulerDriver) Join() (mesos.Status, error) {
	s.JoinCount++
	return s.JoinStatus, s.JoinError
}

func (s *MockSchedulerDriver) Run() (mesos.Status, error) {
	s.RunCount++
	return s.RunStatus, s.RunError
}

func (s *MockSchedulerDriver) RequestResources(requests []*mesos.Request) (mesos.Status, error) {
	s.RequestResourcesCount++
	return s.RequestResourcesStatus, s.RequestResourcesError
}

func (s *MockSchedulerDriver) LaunchTasks(offerIDs []*mesos.OfferID, tasks []*mesos.TaskInfo, filters *mesos.Filters) (mesos.Status, error) {
	s.LaunchTasksCount++
	return s.LaunchTasksStatus, s.LaunchTasksError
}

func (s *MockSchedulerDriver) KillTask(taskID *mesos.TaskID) (mesos.Status, error) {
	s.KillTaskCount++
	return s.KillTaskStatus, s.KillTaskError
}

func (s *MockSchedulerDriver) DeclineOffer(offerID *mesos.OfferID, filters *mesos.Filters) (mesos.Status, error) {
	s.DeclineOfferCount++
	return s.DeclineOfferStatus, s.DeclineOfferError
}

func (s *MockSchedulerDriver) ReviveOffers() (mesos.Status, error) {
	s.ReviveOffersCount++
	return s.ReviveOffersStatus, s.ReviveOffersError
}

func (s *MockSchedulerDriver) SendFrameworkMessage(executorID *mesos.ExecutorID, slaveID *mesos.SlaveID, data string) (mesos.Status, error) {
	s.SendFrameworkMessageCount++
	return s.SendFrameworkMessageStatus, s.SendFrameworkMessageError
}

func (s *MockSchedulerDriver) ReconcileTasks(statuses []*mesos.TaskStatus) (mesos.Status, error) {
	s.ReconcileTasksCount++
	return s.ReconcileTasksStatus, s.ReconcileTasksError
}
