package framework

import (
	"fmt"

	marathon "github.com/gambol99/go-marathon"
	yaml "gopkg.in/yaml.v2"
	"regexp"
)

const (
	ConfigStack = iota
	ConfigDataCenter
	ConfigCluster
	ConfigZone
)

type Stack struct {
	Namespace    string
	Name         string                  `yaml:"name,omitempty"`
	From         string                  `yaml:"from,omitempty"`
	Applications map[string]*Application `yaml:"applications,omitempty"`
	Layer        int

	stateStorage StateStorage
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

func (s *Stack) Run(request *RunRequest, context *Context, client marathon.Marathon, stateStorage StateStorage) (*Context, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	s.stateStorage = stateStorage

	runningApps := make(map[string]ApplicationState)
	statuses := make(chan *applicationRunStatus, len(s.Applications))

	info, err := client.Info()
	if err != nil {
		Logger.Debug("Error getting client info: %s", err)
		return nil, err
	}
	context.Set("mesos.master", info.MarathonConfig.Master)
	context.Set("zone", request.Zone)
	context.Set("stack", s.Name)

	err = s.markSkippedApps(request.SkipApplications, runningApps, statuses)
	if err != nil {
		return nil, err
	}
	s.runApplications(runningApps, context, client, statuses, request.MaxWait)

	for status := range statuses {
		if status.err != nil {
			Logger.Warn("Application %s failed with error %s", status.application.ID, status.err)
			s.stateStorage.SaveApplicationState(status.application.ID, s.ID(), StateFailed)
			return nil, fmt.Errorf("%s: %s", status.application.ID, status.err)
		}

		runningApps[status.application.ID] = StateRunning
		s.stateStorage.SaveApplicationState(status.application.ID, s.ID(), StateRunning)
		if s.allApplicationsRunning(runningApps) {
			close(statuses)
			s.stateStorage.SaveStackState(s.ID(), StateRunning)
			return context, nil
		}

		s.runApplications(runningApps, context, client, statuses, request.MaxWait)
	}

	return context, nil
}

func (s Stack) ID() string {
	return fmt.Sprintf("%s.%s", s.Namespace, s.Name)
}

func (s *Stack) markSkippedApps(skipApplications []string, runningApps map[string]ApplicationState,
	statuses chan *applicationRunStatus) error {
	for _, skipRegex := range skipApplications {
		pattern, err := regexp.Compile(skipRegex)
		if err != nil {
			return err
		}

		for _, app := range s.Applications {
			if pattern.MatchString(app.ID) {
				Logger.Info("Application %s matches skip pattern \"%s\", skipping", app.ID, skipRegex)
				runningApps[app.ID] = StateRunning
				statuses <- newApplicationRunStatus(app, nil)
			}
		}
	}

	return nil
}

func (s *Stack) runApplications(runningApps map[string]ApplicationState, context *Context, client marathon.Marathon,
	status chan *applicationRunStatus, maxWait int) {
	Logger.Debug("Running applications...")
	for _, app := range s.Applications {
		if state, exists := runningApps[app.ID]; exists {
			Logger.Debug("Application %s is in state %s, continuing", app.ID, ApplicationStates[state])
			continue
		}

		if app.IsDependencySatisfied(runningApps) {
			runningApps[app.ID] = StateStaging
			go s.runApplication(app, context, client, status, maxWait)
		}
	}
}

func (s *Stack) runApplication(app *Application, context *Context, client marathon.Marathon,
	status chan *applicationRunStatus, maxWait int) {
	err := app.Run(context, client, s.stateStorage, maxWait)
	if err != nil {
		// TODO should remove the application if anything goes wrong
	}

	status <- newApplicationRunStatus(app, err)
}

func (s *Stack) allApplicationsRunning(apps map[string]ApplicationState) bool {
	for _, app := range s.Applications {
		state, exists := apps[app.ID]
		if !exists || state != StateRunning {
			Logger.Debug("Application %s is not yet running", app.ID)
			return false
		}
	}

	return true
}

type applicationRunStatus struct {
	application *Application
	err         error
}

func newApplicationRunStatus(app *Application, err error) *applicationRunStatus {
	return &applicationRunStatus{
		application: app,
		err:         err,
	}
}
