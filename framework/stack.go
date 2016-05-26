package framework

import (
	"fmt"

	yaml "gopkg.in/yaml.v2"
	"regexp"
)

const (
	ConfigStack = iota
	ConfigDataCenter
	ConfigCluster
	ConfigZone
)

type StackStatus int

const (
	StackStatusStaging StackStatus = iota
	StackStatusRunning
	StackStatusFailed
)

type Stack struct {
	Namespace    string
	Name         string                  `yaml:"name,omitempty"`
	From         string                  `yaml:"from,omitempty"`
	Applications map[string]*Application `yaml:"applications,omitempty"`
	Layer        int
}

func UnmarshalStack(yml []byte) (*Stack, error) {
	Logger.Info("Unmarshalling stack")
	stack := new(Stack)
	err := yaml.Unmarshal(yml, &stack)
	if err != nil {
		return nil, err
	}

	return stack, nil
}

func (s *Stack) GetApplications() map[string]*Application {
	return s.Applications
}

func (s *Stack) GetRunner() Runner {
	return s
}

func (s *Stack) GetStack() *Stack {
	return s
}

func (s *Stack) Merge(child *Stack) {
	Logger.Debug("Merging stacks: \n%s\n\n%s", s, child)
	s.Name = child.Name

	for name, childApp := range child.Applications {
		app, ok := s.Applications[name]
		if !ok {
			s.Applications[name] = childApp
			continue
		}

		setString(childApp.ID, &app.ID)
		setString(childApp.Version, &app.Version)
		setFloat(childApp.Cpu, &app.Cpu)
		setFloat(childApp.Mem, &app.Mem)
		setIntSlice(childApp.Ports, &app.Ports)
		setString(childApp.User, &app.User)
		setString(childApp.Healthcheck, &app.Healthcheck)
		setString(childApp.LaunchCommand, &app.LaunchCommand)
		setStringSlice(childApp.ArtifactURLs, &app.ArtifactURLs)
		setStringSlice(childApp.AdditionalArtifacts, &app.AdditionalArtifacts)
		setStringSlice(childApp.Dependencies, &app.Dependencies)
		for k, v := range childApp.Scheduler {
			if v == "" {
				delete(app.Scheduler, k)
				continue
			}

			if app.Scheduler == nil {
				app.Scheduler = make(map[string]string)
			}
			app.Scheduler[k] = v
		}
		if len(childApp.Tasks) > 0 {
			app.Tasks = childApp.Tasks
		}

		setStringSlice(childApp.BeforeScheduler, &app.BeforeScheduler)
		setStringSlice(childApp.AfterScheduler, &app.AfterScheduler)
		setStringSlice(childApp.BeforeTask, &app.BeforeTask)
		setStringSlice(childApp.AfterTask, &app.AfterTask)
		setStringSlice(childApp.AfterTasks, &app.AfterTasks)
	}
}

func (s *Stack) String() string {
	yml, err := yaml.Marshal(s)
	if err != nil {
		panic(err)
	}

	return string(yml)
}

func (s *Stack) Validate() error {
	//TODO determine circular dependencies also

	// validate applications
	for name, app := range s.Applications {
		if err := app.Validate(); err != nil {
			return fmt.Errorf("Invalid application %s: %s", name, err)
		}
	}

	return nil
}

func (s *Stack) Run(request *RunRequest, context *RunContext) error {
	if err := s.Validate(); err != nil {
		return err
	}

	runningApps := make(map[string]ApplicationStatus)
	statuses := make(chan *ApplicationRunStatus, len(s.Applications))

	info, err := context.Marathon.Info()
	if err != nil {
		Logger.Debug("Error getting client info: %s", err)
		return err
	}
	context.Variables.SetStackVariable("mesos.master", info.MarathonConfig.Master)
	context.Variables.SetStackVariable("zone", context.Zone)
	context.Variables.SetStackVariable("stack", context.StackName)

	err = s.markSkippedApps(request.SkipApplications, runningApps, statuses)
	if err != nil {
		return err
	}
	s.runApplications(runningApps, context, statuses, request.MaxWait)

	for status := range statuses {
		if status.Error != nil {
			Logger.Warn("Application %s failed with error %s", status.Application.ID, status.Error)
			_ = context.Storage.SaveApplicationStatus(context.StackName, context.Zone, status.Application.ID, ApplicationStatusFailed)
			return fmt.Errorf("%s: %s", status.Application.ID, status.Error)
		}

		runningApps[status.Application.ID] = ApplicationStatusRunning
		err = context.Storage.SaveApplicationStatus(context.StackName, context.Zone, status.Application.ID, ApplicationStatusRunning)
		if err != nil {
			return err
		}
		err = context.Storage.SaveStackVariables(context.StackName, context.Zone, context.Variables)
		if err != nil {
			return err
		}
		if s.allApplicationsRunning(runningApps) {
			close(statuses)
			return nil
		}

		s.runApplications(runningApps, context, statuses, request.MaxWait)
	}

	return nil
}

func (s Stack) ID() string {
	return fmt.Sprintf("%s.%s", s.Namespace, s.Name)
}

func (s *Stack) markSkippedApps(skipApplications []string, runningApps map[string]ApplicationStatus,
	statuses chan *ApplicationRunStatus) error {
	for _, skipRegex := range skipApplications {
		pattern, err := regexp.Compile(skipRegex)
		if err != nil {
			return err
		}

		for _, app := range s.Applications {
			if pattern.MatchString(app.ID) {
				Logger.Info("Application %s matches skip pattern \"%s\", skipping", app.ID, skipRegex)
				runningApps[app.ID] = ApplicationStatusRunning
				statuses <- NewApplicationRunStatus(app, nil)
			}
		}
	}

	return nil
}

func (s *Stack) runApplications(runningApps map[string]ApplicationStatus, context *RunContext, status chan *ApplicationRunStatus, maxWait int) {
	Logger.Debug("Running applications...")
	for _, app := range s.Applications {
		if state, exists := runningApps[app.ID]; exists {
			Logger.Debug("Application %s is in state %s, continuing", app.ID, ApplicationStatuses[state])
			continue
		}

		if app.IsDependencySatisfied(runningApps) {
			runningApps[app.ID] = ApplicationStatusStaging
			go s.runApplication(app, context, status, maxWait)
		}
	}
}

func (s *Stack) runApplication(app *Application, context *RunContext, status chan *ApplicationRunStatus, maxWait int) {
	err := app.Run(context, maxWait)
	if err != nil {
		// TODO should remove the application if anything goes wrong
	}

	status <- NewApplicationRunStatus(app, err)
}

func (s *Stack) allApplicationsRunning(apps map[string]ApplicationStatus) bool {
	for _, app := range s.Applications {
		state, exists := apps[app.ID]
		if !exists || state != ApplicationStatusRunning {
			Logger.Debug("Application %s is not yet running", app.ID)
			return false
		}
	}

	return true
}
