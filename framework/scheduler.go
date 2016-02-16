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
	"github.com/mesos/mesos-go/scheduler"
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
}

type StackDeployScheduler struct {
	*SchedulerConfig

	driver scheduler.SchedulerDriver
}

func NewScheduler(config *SchedulerConfig) *StackDeployScheduler {
	return &StackDeployScheduler{
		SchedulerConfig: config,
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
		driver.DeclineOffer(offer.GetId(), &mesos.Filters{RefuseSeconds: proto.Float64(10)})
	}
}

func (s *StackDeployScheduler) OfferRescinded(driver scheduler.SchedulerDriver, id *mesos.OfferID) {
	Logger.Info("[OfferRescinded] %s", id.GetValue())
}

func (s *StackDeployScheduler) StatusUpdate(driver scheduler.SchedulerDriver, status *mesos.TaskStatus) {
	Logger.Info("[StatusUpdate] %s", pretty.Status(status))
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
