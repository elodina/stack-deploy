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
	util "github.com/mesos/mesos-go/mesosutil"
	"github.com/mesos/mesos-go/scheduler"
	"strings"
	"sync"
	"time"
)

type SchedulerConfig struct {
	Master          string
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
}

type StackDeployScheduler struct {
	*SchedulerConfig

	driver          scheduler.SchedulerDriver
	applications    map[string]*MesosApplicationContext
	applicationLock sync.RWMutex
}

func NewScheduler(config *SchedulerConfig) *StackDeployScheduler {
	return &StackDeployScheduler{
		SchedulerConfig: config,
		applications:    make(map[string]*MesosApplicationContext),
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

	return nil
}

func (s *StackDeployScheduler) Registered(driver scheduler.SchedulerDriver, id *mesos.FrameworkID, master *mesos.MasterInfo) {
	Logger.Info("[Registered] framework: %s master: %s:%d", id.GetValue(), master.GetHostname(), master.GetPort())

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

	s.applicationLock.RLock()
	defer s.applicationLock.RUnlock()

	applicationID := applicationIDFromTaskID(status.GetTaskId().GetValue())
	ctx, exists := s.applications[applicationID]
	if !exists {
		Logger.Warn("Got unexpected task state %s for application %s", pretty.Status(status), applicationID)
	}

	if ctx.StatusUpdate(driver, status) {
		delete(s.applications, applicationID)
	}
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
	s.applicationLock.Lock()
	statusChan := make(chan *ApplicationRunStatus)

	// first check if we already have such application, and if so return an error immediately
	_, exists := s.applications[application.ID]
	s.applicationLock.Unlock()
	if exists {
		go func() {
			statusChan <- NewApplicationRunStatus(application, errors.New("Application already exists"))
		}()
		return statusChan
	}

	// if number of slaves is less than number of applications to run return an error immediately
	// TODO this should be removed once we support constraints, now it's like hardcoded hostname=unique
	slaves := Mesos.GetSlaves()
	if len(slaves) < application.getInstances() {
		go func() {
			statusChan <- NewApplicationRunStatus(application, errors.New("Number of instances exceeds available slaves number"))
		}()
		return statusChan
	}

	s.stageApplication(application, statusChan)
	return statusChan
}

func (s *StackDeployScheduler) stageApplication(application *Application, statusChan chan *ApplicationRunStatus) {
	s.applicationLock.Lock()
	defer s.applicationLock.Unlock()

	ctx := NewMesosApplicationContext()
	ctx.Application = application
	ctx.State = StateIdle
	ctx.StatusChan = statusChan
	ctx.InstancesLeftToRun = application.getInstances()

	s.applications[application.ID] = ctx
}

func (s *StackDeployScheduler) acceptOffer(driver scheduler.SchedulerDriver, offer *mesos.Offer) string {
	declineReasons := make([]string, 0)

	applications := s.applicationsWithState(StateIdle)
	if len(applications) == 0 {
		return "all tasks are running"
	}

	for _, ctx := range applications {
		declineReason := ctx.Matches(offer)
		if declineReason == "" {
			ctx.LaunchTask(driver, offer)
			return ""
		} else {
			declineReasons = append(declineReasons, declineReason)
		}
	}

	return strings.Join(declineReasons, ", ")
}

func (s *StackDeployScheduler) applicationsWithState(state ApplicationState) []*MesosApplicationContext {
	s.applicationLock.RLock()
	defer s.applicationLock.RUnlock()

	applications := make([]*MesosApplicationContext, 0)
	for id, applicationContext := range s.applications {
		if applicationContext.State == state {
			applications = append(applications, s.applications[id])
		}
	}

	return applications
}

func getScalarResources(offer *mesos.Offer, resourceName string) float64 {
	resources := 0.0
	filteredResources := util.FilterResources(offer.Resources, func(res *mesos.Resource) bool {
		return res.GetName() == resourceName
	})
	for _, res := range filteredResources {
		resources += res.GetScalar().GetValue()
	}
	return resources
}
