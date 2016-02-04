package framework

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	marathon "github.com/gambol99/go-marathon"
	yaml "gopkg.in/yaml.v2"
	"io"
	"strconv"
)

type ApplicationState int

const (
	StateStaging ApplicationState = iota
	StateRunning
	StateFail
)

// exposed for testing purposes
var stdout io.Writer = os.Stdout
var applicationAwaitBackoff = time.Second

var variableRegexp = regexp.MustCompile("\\$\\{.*\\}")

type Application struct {
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
	ArtifactURLs        []string          `yaml:"artifact_urls,omitempty"`
	AdditionalArtifacts []string          `yaml:"additional_artifacts,omitempty"`
	Scheduler           map[string]string `yaml:"scheduler,omitempty"`
	Tasks               yaml.MapSlice     `yaml:"tasks,omitempty"`
	Dependencies        []string          `yaml:"dependencies,omitempty"`

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

	if a.LaunchCommand == "" {
		return ErrApplicationNoLaunchCommand
	}

	if a.Instances != "" && a.Instances != "all" {
		instances, err := strconv.Atoi(a.Instances)
		if err != nil || instances < 1 {
			return ErrApplicationInvalidInstances
		}
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

func (a *Application) Run(context *Context, client marathon.Marathon, stateStorage StateStorage, maxWait int) error {
	Logger.Debug("Running application: \n%s", a)
	a.stateStorage = stateStorage
	a.resolveVariables(context)
	err := ensureVariablesResolved(context, a.BeforeScheduler, a.LaunchCommand, a.Scheduler)
	if err != nil {
		return err
	}
	err = a.executeCommands(a.BeforeScheduler, fmt.Sprintf("%s_before_scheduler.sh", a.ID))
	if err != nil {
		return err
	}

	application := a.createApplication()
	_, err = client.CreateApplication(application)
	if err != nil {
		return err
	}

	err = a.awaitRunningAndHealthy(client, maxWait)
	if err != nil {
		return err
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

func (a *Application) storeTaskState(task map[string]string, context *Context) error {
	err := a.stateStorage.SaveTaskState(task, context.All(), StateRunning)
	if err != nil {
		Logger.Error(err)
	}
	return err
}

func (a *Application) resolveVariables(context *Context) {
	for k, v := range context.All() {
		a.LaunchCommand = strings.Replace(a.LaunchCommand, fmt.Sprintf("${%s}", fmt.Sprint(k)), fmt.Sprint(v), -1)
		for schedulerKey, schedulerValue := range a.Scheduler {
			a.Scheduler[schedulerKey] = strings.Replace(schedulerValue, fmt.Sprintf("${%s}", fmt.Sprint(k)), fmt.Sprint(v), -1)
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

func (a *Application) resolveCmdVariables(commands []string, context *Context) {
	for k, v := range context.All() {
		for idx, cmd := range commands {
			commands[idx] = strings.Replace(cmd, fmt.Sprintf("${%s}", k), v, -1)
		}
	}
}

func (a *Application) fillContext(context *Context, runner TaskRunner, client marathon.Marathon) error {
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

func (a *Application) createApplication() *marathon.Application {
	application := &marathon.Application{
		ID:           a.ID,
		Cmd:          a.getLaunchCommand(),
		Instances:    a.getInstances(),
		CPUs:         a.Cpu,
		Mem:          a.Mem,
		Ports:        a.Ports,
		RequirePorts: len(a.Ports) > 0,
		Uris:         append(a.ArtifactURLs, a.AdditionalArtifacts...),
		User:         a.User,
		HealthChecks: a.getHealthchecks(),
		Constraints:  a.Constraints,
	}
	return application
}

func (a *Application) getLaunchCommand() string {
	cmd := a.LaunchCommand
	for k, v := range a.Scheduler {
		cmd += fmt.Sprintf(" --%s %s", k, fmt.Sprint(v))
	}

	return cmd
}

func (a *Application) getInstances() int {
	if a.Instances == "" {
		return 1
	}

	if a.Instances == "all" {
		return int(Mesos.GetActivatedSlaves())
	}

	instances, err := strconv.Atoi(a.Instances)
	if err != nil {
		// should not happen, must be validated first
		panic(err)
	}

	return instances
}

func (a *Application) getHealthchecks() []*marathon.HealthCheck {
	if a.Healthcheck != "" {
		return []*marathon.HealthCheck{
			&marathon.HealthCheck{
				Protocol:               "HTTP",
				Path:                   a.Healthcheck,
				GracePeriodSeconds:     120,
				IntervalSeconds:        60,
				PortIndex:              0,
				MaxConsecutiveFailures: 3,
				TimeoutSeconds:         30,
			},
		}
	}

	return nil
}

func (a *Application) String() string {
	yml, err := yaml.Marshal(a)
	if err != nil {
		panic(err)
	}

	return string(yml)
}

func ensureVariablesResolved(context *Context, values ...interface{}) error {
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

func ensureStringVariableResolved(context *Context, value string) error {
	unresolved := variableRegexp.FindString(value)
	if unresolved != "" {
		return fmt.Errorf("Unresolved variable %s. Available variables:\n%s", unresolved, context)
	}

	return nil
}
