package mesosrunners

import (
	"github.com/elodina/stack-deploy/framework"
)

func (r *RunOnceRunner) ScheduleApplication(application *framework.Application, state framework.MesosState, cronScheduler framework.CronScheduler) (int64, <-chan *framework.ApplicationRunStatus) {
	var id int64
	if application.StartTime != "" && application.TimeSchedule != "" {
		id = r.StartWithSchedule(application, state, cronScheduler)
	}
	if application.StartTime != "" && application.TimeSchedule == "" {
		id = r.StartOnly(application, state, cronScheduler)
	}
	if application.StartTime == "" && application.TimeSchedule != "" {
		id = r.ScheduleOnly(application, state, cronScheduler)
	}

	ch := make(chan *framework.ApplicationRunStatus, 1)
	ch <- framework.NewApplicationRunStatus(application, nil)
	return id, ch
}

func (r *RunOnceRunner) DeleteSchedule(id int64, cronScheduler framework.CronScheduler) {
	cronScheduler.DeleteJob(id)
}

func (r *RunOnceRunner) StartWithSchedule(application *framework.Application, state framework.MesosState, cronScheduler framework.CronScheduler) int64 {
	id, _ := cronScheduler.AddFunc(application.StartTime, func() {
		cronScheduler.AddFunc(application.TimeSchedule, func() {
			<-r.StageApplication(application, state)
		})
	})
	return id
}

func (r *RunOnceRunner) StartOnly(application *framework.Application, state framework.MesosState, cronScheduler framework.CronScheduler) int64 {
	id, _ := cronScheduler.AddFunc(application.StartTime, func() {
		<-r.StageApplication(application, state)
	})
	return id
}

func (r *RunOnceRunner) ScheduleOnly(application *framework.Application, state framework.MesosState, cronScheduler framework.CronScheduler) int64 {
	id, _ := cronScheduler.AddFunc(application.TimeSchedule, func() {
		<-r.StageApplication(application, state)
	})
	return id
}
