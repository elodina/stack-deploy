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

	"errors"
	"github.com/elodina/go-mesos-utils/pretty"
	"github.com/golang/protobuf/proto"
	mesos "github.com/mesos/mesos-go/mesosproto"
	"github.com/mesos/mesos-go/mesosutil"
	"github.com/mesos/mesos-go/scheduler"
	"github.com/yanzay/cron"
	"strings"
	"time"
)

type SchedulerConfig struct {
	Master          string
	Storage         *FrameworkStorage
	User            string
	FrameworkName   string
	FrameworkRole   string
	FailoverTimeout time.Duration
}

func NewSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		FrameworkName:   "stack-deploy",
		FrameworkRole:   "*",
		FailoverTimeout: 168 * time.Hour, // 1 week
	}
}

type Scheduler interface {
	Start() error
	RunApplication(application *Application) <-chan *ApplicationRunStatus
	GetMesosState() MesosState
	GetScheduledTasks() []*ScheduledTask
	RemoveScheduled(int64) bool
}

type StackDeployScheduler struct {
	*SchedulerConfig
	driver         scheduler.SchedulerDriver
	state          MesosState
	Cron           *cron.Cron
	scheduledTasks []*ScheduledTask
}

func NewScheduler(config *SchedulerConfig) *StackDeployScheduler {
	return &StackDeployScheduler{
		SchedulerConfig: config,
		state:           NewMesosClusterState(config.Master),
		Cron:            cron.New(),
		scheduledTasks:  make([]*ScheduledTask, 0),
	}
}

func (s *StackDeployScheduler) Start() error {
	Logger.Info("Starting scheduler")

	frameworkInfo := &mesos.FrameworkInfo{
		User:            proto.String(s.User),
		Name:            proto.String(s.FrameworkName),
		Role:            proto.String(s.FrameworkRole),
		FailoverTimeout: proto.Float64(float64(s.FailoverTimeout / 1e9)),
		Checkpoint:      proto.Bool(true),
	}

	if s.Storage.FrameworkID != "" {
		frameworkInfo.Id = mesosutil.NewFrameworkID(s.Storage.FrameworkID)
	}

	driverConfig := scheduler.DriverConfig{
		Scheduler: s,
		Framework: frameworkInfo,
		Master:    s.Master,
	}

	driver, err := scheduler.NewMesosSchedulerDriver(driverConfig)

	if err != nil {
		return fmt.Errorf("Unable to create SchedulerDriver: %s", err)
	}

	go func() {
		if stat, err := driver.Run(); err != nil {
			Logger.Info("Framework stopped with status %s and error: %s", stat.String(), err)
			panic(err)
		}
	}()

	s.Cron.Start()
	go func() {
		for {
			Logger.Info("Cron entries: %v\n", s.Cron.Entries())
			for _, entry := range s.Cron.Entries() {
				Logger.Info("Entry: %v, %v, %v, %v", entry.Prev, entry.Next, entry.Schedule, entry.Job)
			}
			time.Sleep(10 * time.Second)
		}
	}()

	return nil
}

type ScheduledTask struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	StartTime    string `json:"start_time"`
	TimeSchedule string `json:"time_schedule"`
}

func (s *StackDeployScheduler) GetScheduledTasks() []*ScheduledTask {
	return s.scheduledTasks
}

func (s *StackDeployScheduler) RemoveScheduled(id int64) bool {
	runner := MesosTaskRunners["run-once"]
	Logger.Info("Deleting scheduled task: %d", id)
	runner.DeleteSchedule(id, s.Cron)
	for i, task := range s.scheduledTasks {
		Logger.Info("%d == %d: %t", task.ID, id, task.ID == id)
		if task.ID == id {
			Logger.Info("Task %d found. Removing from list", id)
			s.scheduledTasks = append(s.scheduledTasks[:i], s.scheduledTasks[i+1:]...)
			return true
		}
	}
	return false
}

func (s *StackDeployScheduler) Registered(driver scheduler.SchedulerDriver, id *mesos.FrameworkID, master *mesos.MasterInfo) {
	Logger.Info("[Registered] framework: %s master: %s:%d", id.GetValue(), master.GetHostname(), master.GetPort())

	s.Storage.FrameworkID = id.GetValue()
	s.Storage.Save()
	s.driver = driver
}

func (s *StackDeployScheduler) Reregistered(driver scheduler.SchedulerDriver, master *mesos.MasterInfo) {
	Logger.Info("[Reregistered] master: %s:%d", master.GetHostname(), master.GetPort())

	s.driver = driver
}

func (s *StackDeployScheduler) Disconnected(scheduler.SchedulerDriver) {
	Logger.Info("[Disconnected]")

	s.driver = nil
}

func (s *StackDeployScheduler) ResourceOffers(driver scheduler.SchedulerDriver, offers []*mesos.Offer) {
	Logger.Debug("[ResourceOffers] %s", pretty.Offers(offers))

	for _, offer := range offers {
		declineReason := s.acceptOffer(driver, offer)
		if declineReason != "" {
			driver.DeclineOffer(offer.GetId(), &mesos.Filters{RefuseSeconds: proto.Float64(10)})
			Logger.Debug("Declined offer %s: %s", pretty.Offer(offer), declineReason)
		}
	}
}

func (s *StackDeployScheduler) OfferRescinded(driver scheduler.SchedulerDriver, id *mesos.OfferID) {
	Logger.Info("[OfferRescinded] %s", id.GetValue())
}

func (s *StackDeployScheduler) StatusUpdate(driver scheduler.SchedulerDriver, status *mesos.TaskStatus) {
	Logger.Info("[StatusUpdate] %s", pretty.Status(status))

	if status.GetState() == mesos.TaskState_TASK_FINISHED {
		driver.ReviveOffers()
	}

	for _, runner := range MesosTaskRunners {
		if runner.StatusUpdate(driver, status) {
			return
		}
	}

	Logger.Warn("Received status update that was not handled by any Mesos Task Runner: %s", pretty.Status(status))
}

func (s *StackDeployScheduler) FrameworkMessage(driver scheduler.SchedulerDriver, executor *mesos.ExecutorID, slave *mesos.SlaveID, message string) {
	Logger.Info("[FrameworkMessage] executor: %s slave: %s message: %s", executor, slave, message)
}

func (s *StackDeployScheduler) SlaveLost(driver scheduler.SchedulerDriver, slave *mesos.SlaveID) {
	Logger.Info("[SlaveLost] %s", slave.GetValue())
}

func (s *StackDeployScheduler) ExecutorLost(driver scheduler.SchedulerDriver, executor *mesos.ExecutorID, slave *mesos.SlaveID, status int) {
	Logger.Info("[ExecutorLost] executor: %s slave: %s status: %d", executor, slave, status)
}

func (s *StackDeployScheduler) Error(driver scheduler.SchedulerDriver, message string) {
	Logger.Error("[Error] %s", message)
}

func (s *StackDeployScheduler) Shutdown(driver *scheduler.MesosSchedulerDriver) {
	Logger.Info("Shutdown triggered, stopping driver")
	driver.Stop(false)
}

func (s *StackDeployScheduler) RunApplication(application *Application) <-chan *ApplicationRunStatus {
	Logger.Debug("Scheduler received run request for application %s", application.ID)
	statusChan := make(chan *ApplicationRunStatus)

	runner, exists := MesosTaskRunners[application.Type]
	if !exists {
		go func() {
			statusChan <- NewApplicationRunStatus(application, errors.New("Application already exists"))
		}()
		return statusChan
	}

	if application.TimeSchedule != "" || application.StartTime != "" {
		id, ch := runner.ScheduleApplication(application, s.GetMesosState(), s.Cron)
		task := &ScheduledTask{
			ID:           id,
			Name:         application.ID,
			TimeSchedule: application.TimeSchedule,
			StartTime:    application.StartTime,
		}
		s.scheduledTasks = append(s.scheduledTasks, task)
		return ch
	}

	return runner.StageApplication(application, s.GetMesosState())
}

func (s *StackDeployScheduler) GetMesosState() MesosState {
	return s.state
}

func (s *StackDeployScheduler) acceptOffer(driver scheduler.SchedulerDriver, offer *mesos.Offer) string {
	declineReasons := make([]string, 0)

	for name, runner := range MesosTaskRunners {
		declineReason, err := runner.ResourceOffer(driver, offer)
		if err != nil {
			Logger.Warn("Error during processing resource offer %s by Mesos Task Runner '%s': %s", pretty.Offer(offer), name, err)
			continue
		}

		if declineReason != "" {
			declineReasons = append(declineReasons, declineReason)
		}
	}

	return strings.Join(declineReasons, ", ")
}
