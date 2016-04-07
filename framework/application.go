package framework

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/elodina/stack-deploy/constraints"
	marathon "github.com/gambol99/go-marathon"
	"github.com/yanzay/log"
	yaml "gopkg.in/yaml.v2"
)

type ApplicationState int

const (
	StateStaging ApplicationState = iota
	StateRunning
	StateFailed
)

var ApplicationStates = map[ApplicationState]string{
	StateStaging: "STAGING",
	StateRunning: "RUNNING",
	StateFailed:  "FAILED",
}

// exposed for testing purposes
var stdout io.Writer = os.Stdout
var applicationAwaitBackoff = time.Second

var variableRegexp = regexp.MustCompile("\\$\\{.*\\}")

type Application struct {
	sync.RWMutex
	Type                string            `yaml:"type,omitempty"`
	ID                  string            `yaml:"id,omitempty"`
	Version             string            `yaml:"version,omitempty"`
	Cpu                 float64           `yaml:"cpu,omitempty"`
	Mem                 float64           `yaml:"mem,omitempty"`
	Ports               []int             `yaml:"ports,omitempty"`
	Instances           string            `yaml:"instances,omitempty"`
	Constraints         [][]string        `yaml:"constraints,omitempty"`
	User                string            `yaml:"user,omitempty"`
	Healthcheck         string            `yaml:"healthcheck,omitempty"`
	LaunchCommand       string            `yaml:"launch_command,omitempty"`
	Args                []string          `yaml:"args,omitempty"`
	Env                 map[string]string `yaml:"env,omitempty"`
	ArtifactURLs        []string          `yaml:"artifact_urls,omitempty"`
	AdditionalArtifacts []string          `yaml:"additional_artifacts,omitempty"`
	Scheduler           map[string]string `yaml:"scheduler,omitempty"`
	Tasks               yaml.MapSlice     `yaml:"tasks,omitempty"`
	Dependencies        []string          `yaml:"dependencies,omitempty"`
	Docker              *Docker           `yaml:"docker,omitempty"`
	StartTime           string            `yaml:"start_time,omitempty"`
	TimeSchedule        string            `yaml:"time_schedule",omitempty"`

	BeforeScheduler []string `yaml:"before_scheduler,omitempty"`
	AfterScheduler  []string `yaml:"after_scheduler,omitempty"`
	BeforeTask      []string `yaml:"before_task,omitempty"`
	AfterTask       []string `yaml:"after_task,omitempty"`
	AfterTasks      []string `yaml:"after_tasks,omitempty"`

	stateStorage StateStorage
}

func (a *Application) Validate() error {
	if a.Type == "" {
		return ErrApplicationNoType
	}

	if len(a.Tasks) > 0 {
		_, ok := TaskRunners[a.Type]
		if !ok {
			Logger.Info("%s: %s", ErrApplicationNoTaskRunner, a.Type)
			return ErrApplicationNoTaskRunner
		}
	}

	if a.ID == "" {
		return ErrApplicationNoID
	}

	if a.Cpu <= 0.0 {
		return ErrApplicationInvalidCPU
	}

	if a.Mem <= 0.0 {
		return ErrApplicationInvalidMem
	}

	if a.Instances != "" && a.Instances != "all" {
		instances, err := strconv.Atoi(a.Instances)
		if err != nil || instances < 1 {
			return ErrApplicationInvalidInstances
		}
	}

	_, err := constraints.ParseConstraints(a.Constraints)
	if err != nil {
		return err
	}

	return nil
}

func (a *Application) IsDependencySatisfied(runningApps map[string]ApplicationState) bool {
	for _, dependency := range a.Dependencies {
		state, ok := runningApps[dependency]
		if !ok || state != StateRunning {
			Logger.Debug("Application %s has unsatisfied dependency %s", a.ID, dependency)
			return false
		}
	}

	return true
}

func (a *Application) Run(context *StackContext, client marathon.Marathon, scheduler Scheduler, stateStorage StateStorage, maxWait int) error {
	Logger.Debug("Running application: \n%s", a)
	a.stateStorage = stateStorage
	a.resolveVariables(context)
	err := ensureVariablesResolved(context, a.BeforeScheduler, a.LaunchCommand, a.Scheduler, a.Args, a.Env)
	if err != nil {
		return err
	}
	err = a.executeCommands(a.BeforeScheduler, fmt.Sprintf("%s_before_scheduler.sh", a.ID))
	if err != nil {
		return err
	}

	if _, exists := MesosTaskRunners[a.Type]; exists {
		err := a.runMesos(scheduler)
		if err != nil {
			return err
		}
	} else {
		err := a.runMarathon(context, client, scheduler, stateStorage, maxWait)
		if err != nil {
			return err
		}
	}

	a.resolveVariables(context)
	err = ensureVariablesResolved(context, a.AfterScheduler)
	if err != nil {
		return err
	}
	err = a.executeCommands(a.AfterScheduler, fmt.Sprintf("%s_after_scheduler.sh", a.ID))
	if err != nil {
		return err
	}

	runner := TaskRunners[a.Type]
	if runner != nil {
		err = a.fillContext(context, runner, client)
		if err != nil {
			return err
		}
		Logger.Info("Context:\n%s", context)

		for _, task := range a.Tasks {
			a.resolveVariables(context)
			err = ensureVariablesResolved(context, a.BeforeTask, task)
			if err != nil {
				return err
			}
			err = a.executeCommands(a.BeforeTask, fmt.Sprintf("%s_before_task.sh", a.ID))
			if err != nil {
				return err
			}

			Logger.Info("Running task %s", task.Key)
			err = runner.RunTask(context, a, MapSliceToMap(task.Value.(yaml.MapSlice)))
			if err != nil {
				return err
			}

			a.resolveVariables(context)
			err = ensureVariablesResolved(context, a.AfterTask)
			if err != nil {
				return err
			}
			err = a.executeCommands(a.AfterTask, fmt.Sprintf("%s_after_task.sh", a.ID))
			if err != nil {
				return err
			}

			// a.storeTaskState(task, context)
		}
	}

	a.resolveVariables(context)
	err = ensureVariablesResolved(context, a.AfterTasks)
	if err != nil {
		return err
	}
	return a.executeCommands(a.AfterTasks, fmt.Sprintf("%s_after_tasks.sh", a.ID))
}

func (a *Application) runMarathon(context *StackContext, client marathon.Marathon, scheduler Scheduler, stateStorage StateStorage, maxWait int) error {
	application := a.createApplication(context, scheduler.GetMesosState())
	_, err := client.CreateApplication(application)
	if err != nil {
		return err
	}

	err = a.awaitRunningAndHealthy(client, maxWait)
	if err != nil {
		return err
	}

	return nil
}

func (a *Application) runMesos(scheduler Scheduler) error {
	applicationStatuses := scheduler.RunApplication(a)
	status := <-applicationStatuses
	if status.Error != nil {
		return status.Error
	}
	go func() {
		for {
			status = <-applicationStatuses
			if status.Error != nil {
				log.Errorf("Application status error: %s", status.Error)
				return
			}
		}
	}()
	return nil
}

func (a *Application) storeTaskState(task map[string]string, context *StackContext) error {
	err := a.stateStorage.SaveTaskState(task, context.All(), StateRunning)
	if err != nil {
		Logger.Error(err)
	}
	return err
}

func (a *Application) resolveVariables(context *StackContext) {
	a.Lock()
	defer a.Unlock()
	for k, v := range context.All() {
		a.LaunchCommand = strings.Replace(a.LaunchCommand, fmt.Sprintf("${%s}", fmt.Sprint(k)), fmt.Sprint(v), -1)
		for envKey, envValue := range a.Env {
			a.Env[envKey] = strings.Replace(envValue, fmt.Sprintf("${%s}", fmt.Sprint(k)), fmt.Sprint(v), -1)
		}
		for schedulerKey, schedulerValue := range a.Scheduler {
			a.Scheduler[schedulerKey] = strings.Replace(schedulerValue, fmt.Sprintf("${%s}", fmt.Sprint(k)), fmt.Sprint(v), -1)
		}
		for idx, arg := range a.Args {
			a.Args[idx] = strings.Replace(arg, fmt.Sprintf("${%s}", fmt.Sprint(k)), fmt.Sprint(v), -1)
		}
		for _, taskSlice := range a.Tasks {
			if taskSlice.Value != nil {
				tasks := taskSlice.Value.(yaml.MapSlice)
				for i := 0; i < len(tasks); i++ {
					tasks[i].Value = strings.Replace(fmt.Sprint(tasks[i].Value), fmt.Sprintf("${%s}", fmt.Sprint(k)), fmt.Sprint(v), -1)
				}
			}
		}

		a.resolveCmdVariables(a.BeforeScheduler, context)
		a.resolveCmdVariables(a.AfterScheduler, context)
		a.resolveCmdVariables(a.BeforeTask, context)
		a.resolveCmdVariables(a.AfterTask, context)
		a.resolveCmdVariables(a.AfterTasks, context)
	}
}

func (a *Application) executeCommands(commands []string, fileName string) error {
	if len(commands) == 0 {
		Logger.Info("%s has nothing to run, skipping", fileName)
		return nil
	}

	Logger.Info("Running %s", fileName)
	err := ioutil.WriteFile(fileName, []byte(fmt.Sprintf("#!/bin/sh\n\n%s", strings.Join(commands, "\n"))), 0777)
	if err != nil {
		return err
	}
	defer os.Remove(fileName)

	cmd := exec.Command("sh", fileName)
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (a *Application) resolveCmdVariables(commands []string, context *StackContext) {
	for k, v := range context.All() {
		for idx, cmd := range commands {
			commands[idx] = strings.Replace(cmd, fmt.Sprintf("${%s}", k), v, -1)
		}
	}
}

func (a *Application) fillContext(context *StackContext, runner TaskRunner, client marathon.Marathon) error {
	tasks, err := client.Tasks(a.ID)
	if err != nil {
		return err
	}

	if tasks == nil || len(tasks.Tasks) == 0 {
		return ErrTaskNotRunning
	}

	return runner.FillContext(context, a, tasks.Tasks[0])
}

func (a *Application) awaitRunningAndHealthy(client marathon.Marathon, retries int) error {
	for i := 0; i <= retries; i++ {
		err := a.checkRunningAndHealthy(client)
		if err == nil {
			return nil
		}

		time.Sleep(applicationAwaitBackoff)
	}
	return fmt.Errorf("Failed to await until the task is running and healthy within %d retries", retries)
}

func (a *Application) checkRunningAndHealthy(client marathon.Marathon) error {
	app, err := client.Application(a.ID)
	if err != nil {
		return err
	}

	if app == nil {
		return ErrApplicationDoesNotExist
	}

	if app.TasksRunning == 0 {
		return ErrTaskNotRunning
	}

	if a.Healthcheck != "" && app.TasksHealthy == 0 {
		return ErrHealthcheckNotPassing
	}

	return nil
}

func (a *Application) createApplication(context *StackContext, mesos MesosState) *marathon.Application {
	launchCommand := a.getLaunchCommand()
	env := a.getEnv(context)
	instances := a.GetInstances(mesos)
	requirePorts := len(a.Ports) > 0
	uris := append(a.ArtifactURLs, a.AdditionalArtifacts...)
	healthchecks := a.getHealthchecks()
	labels := a.getLabelsFromContext(context)
	application := &marathon.Application{
		ID:           a.ID,
		Cmd:          &launchCommand,
		Args:         &a.Args,
		Env:          &env,
		Instances:    &instances,
		CPUs:         a.Cpu,
		Mem:          &a.Mem,
		Ports:        a.Ports,
		RequirePorts: &requirePorts,
		Uris:         &uris,
		User:         a.User,
		HealthChecks: &healthchecks,
		Constraints:  &a.Constraints,
		Labels:       &labels,
		Container:    a.getContainer(),
	}
	return application
}

func (a *Application) getLabelsFromContext(context *StackContext) map[string]string {
	keys := []string{"zone", "stack"}
	labels := make(map[string]string)
	for _, key := range keys {
		val, _ := context.Get(key)
		if val != "" {
			labels[key] = val
		}
	}
	return labels
}

func (a *Application) getLaunchCommand() string {
	cmd := a.LaunchCommand
	for k, v := range a.Scheduler {
		cmd += fmt.Sprintf(" --%s %s", k, fmt.Sprint(v))
	}
	return cmd
}

func (a *Application) getEnv(context *StackContext) map[string]string {
	env := make(map[string]string)
	labelStrings := make([]string, 0)
	for key, val := range a.getLabelsFromContext(context) {
		labelStrings = append(labelStrings, fmt.Sprintf("%s=%s", key, val))
	}
	stackLabels := strings.Join(labelStrings, ";")
	if stackLabels != "" {
		env["STACK_LABELS"] = stackLabels
	}
	for k, v := range a.Env {
		env[k] = v
	}

	return env
}

func (a *Application) GetInstances(mesos MesosState) int {
	if a.Instances == "" {
		return 1
	}

	if a.Instances == "all" {
		return a.calculateAllInstances(mesos)
	}

	instances, err := strconv.Atoi(a.Instances)
	if err != nil {
		// should not happen, must be validated first
		panic(err)
	}

	return instances
}

func (a *Application) getContainer() *marathon.Container {
	if a.Docker == nil {
		return nil
	}

	return a.Docker.MarathonContainer()
}

func (a *Application) calculateAllInstances(mesos MesosState) int {
	constraints := a.GetConstraints()
	if len(constraints) == 0 {
		Logger.Debug("No constraints, all instances == %d", mesos.GetActivatedSlaves())
		return mesos.GetActivatedSlaves()
	}

	matchingSlaves := make([]Slave, 0)
	for _, slave := range mesos.GetSlaves() {
		if a.slaveMatches(constraints, slave, matchingSlaves) {
			matchingSlaves = append(matchingSlaves, slave)
		}
	}

	return len(matchingSlaves)
}

func (a *Application) slaveMatches(constraints map[string][]constraints.Constraint, slave Slave, otherSlaves []Slave) bool {
	for attribute, attributeConstraints := range constraints {
		slaveAttribute := slave.Attribute(attribute)
		if slaveAttribute == "" {
			Logger.Debug("Slave %s does not have attribute %s, thus does not match constraints", slave.ID, attribute)
			return false
		}

		for _, constraint := range attributeConstraints {
			if !constraint.Matches(slaveAttribute, a.otherSlavesAttributes(otherSlaves, attribute)) {
				Logger.Debug("Slave %s does not match constraint %s", slave.ID, constraint)
				return false
			}
		}
	}

	Logger.Debug("Slave %s matches constraints", slave.ID)
	return true
}

func (a *Application) otherSlavesAttributes(slaves []Slave, name string) []string {
	attributes := make([]string, 0)
	for _, slave := range slaves {
		attribute := slave.Attribute(name)
		if attribute != "" {
			attributes = append(attributes, attribute)
		}
	}

	return attributes
}

func (a *Application) GetConstraints() map[string][]constraints.Constraint {
	constraints, err := constraints.ParseConstraints(a.Constraints)
	if err != nil {
		panic(err) //constraints should be validated before this call
	}
	return constraints
}

func (a *Application) getHealthchecks() []marathon.HealthCheck {
	if a.Healthcheck != "" {
		portIndex := 0
		maxFailures := 18
		return []marathon.HealthCheck{
			marathon.HealthCheck{
				Protocol:               "HTTP",
				Path:                   &a.Healthcheck,
				GracePeriodSeconds:     120,
				IntervalSeconds:        10,
				PortIndex:              &portIndex,
				MaxConsecutiveFailures: &maxFailures,
				TimeoutSeconds:         30,
			},
		}
	}

	return nil
}

func (a *Application) String() string {
	a.RLock()
	yml, err := yaml.Marshal(a)
	a.RUnlock()
	if err != nil {
		panic(err)
	}

	return string(yml)
}

func ensureVariablesResolved(context *StackContext, values ...interface{}) error {
	for _, value := range values {
		switch v := value.(type) {
		case string:
			{
				if err := ensureStringVariableResolved(context, v); err != nil {
					return err
				}
			}
		case []string:
			{
				for _, val := range v {
					if err := ensureStringVariableResolved(context, val); err != nil {
						return err
					}
				}
			}
		case map[string]string:
			{
				for _, val := range v {
					if err := ensureStringVariableResolved(context, val); err != nil {
						return err
					}
				}
			}
		case yaml.MapSlice:
			{
				for _, m := range v {
					if err := ensureVariablesResolved(context, m.Value); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func ensureStringVariableResolved(context *StackContext, value string) error {
	unresolved := variableRegexp.FindString(value)
	if unresolved != "" {
		return fmt.Errorf("Unresolved variable %s. Available variables:\n%s", unresolved, context)
	}

	return nil
}

type ApplicationRunStatus struct {
	Application *Application
	Error       error
}

func NewApplicationRunStatus(application *Application, err error) *ApplicationRunStatus {
	return &ApplicationRunStatus{
		Application: application,
		Error:       err,
	}
}
