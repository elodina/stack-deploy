package mesosrunners

import (
	"github.com/elodina/stack-deploy/framework"
)

func (r *RunOnceRunner) ScheduleApplication(application *framework.Application, state framework.MesosState, cronScheduler framework.CronScheduler) <-chan *framework.ApplicationRunStatus {
	if application.StartTime != "" && application.TimeSchedule != "" {
		r.StartWithSchedule(application, state, cronScheduler)
	}
	if application.StartTime != "" && application.TimeSchedule == "" {
		r.StartOnly(application, state, cronScheduler)
	}
	if application.StartTime == "" && application.TimeSchedule != "" {
		r.ScheduleOnly(application, state, cronScheduler)
	}

	ch := make(chan *framework.ApplicationRunStatus, 1)
	ch <- framework.NewApplicationRunStatus(application, nil)
	return ch
}

func (r *RunOnceRunner) StartWithSchedule(application *framework.Application, state framework.MesosState, cronScheduler framework.CronScheduler) {
	cronScheduler.AddFunc(application.StartTime, func() {
		cronScheduler.AddFunc(application.TimeSchedule, func() {
			<-r.StageApplication(application, state)
		})
	})
}

func (r *RunOnceRunner) StartOnly(application *framework.Application, state framework.MesosState, cronScheduler framework.CronScheduler) {
	cronScheduler.AddFunc(application.StartTime, func() {
		<-r.StageApplication(application, state)
	})
}

func (r *RunOnceRunner) ScheduleOnly(application *framework.Application, state framework.MesosState, cronScheduler framework.CronScheduler) {
	cronScheduler.AddFunc(application.TimeSchedule, func() {
		<-r.StageApplication(application, state)
	})
}
