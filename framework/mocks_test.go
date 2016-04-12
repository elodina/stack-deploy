package framework

import (
	"fmt"
	marathon "github.com/gambol99/go-marathon"
)

type MockStorage struct{}

func (*MockStorage) GetAll() ([]*Stack, error) {
	stack := &Stack{
		Name:  "stack1",
		Layer: LayerStack,
	}
	return []*Stack{stack}, nil
}
func (*MockStorage) GetStack(name string) (*Stack, error) {
	if name != "stack1" {
		return nil, fmt.Errorf("Stack not found")
	}
	stack := &Stack{
		Name:  "stack1",
		Layer: LayerStack,
	}
	return stack, nil
}

type FakeStack struct{}

func (*FakeStack) Run(*RunRequest, *StackContext, marathon.Marathon, Scheduler, StateStorage) (*StackContext, error) {
	return &StackContext{}, nil
}
func (*FakeStack) GetStack() *Stack {
	return &Stack{
		Name:  "stack1",
		Layer: LayerStack,
	}
}
func (*FakeStack) Merge(*Stack)      {}
func (*FakeStack) GetRunner() Runner { return &FakeStack{} }

func (ms *MockStorage) GetStackRunner(name string) (Runner, error) {
	return &FakeStack{}, nil
}
func (*MockStorage) StoreStack(*Stack) error               { return nil }
func (*MockStorage) RemoveStack(string, bool) error        { return nil }
func (*MockStorage) Init() error                           { return nil }
func (*MockStorage) GetLayersStack(string) (Merger, error) { return &FakeStack{}, nil }

type MockUserStorage struct{}

func (*MockUserStorage) SaveUser(User) error           { return nil }
func (*MockUserStorage) GetUser(string) (*User, error) { return nil, nil }
func (*MockUserStorage) CheckKey(string, key string) (bool, error) {
	if key == "key" {
		return true, nil
	}
	return false, nil
}
func (*MockUserStorage) IsAdmin(user string) (bool, error) {
	if user == "admin" {
		return true, nil
	}
	return false, nil
}
func (*MockUserStorage) CreateUser(string, UserRole) (string, error) { return "", nil }
func (*MockUserStorage) RefreshToken(string) (string, error)         { return "", nil }

type MockStateStorage struct{}

func (*MockStateStorage) SaveTaskState(map[string]string, map[string]string, ApplicationState) error {
	return nil
}
func (*MockStateStorage) SaveApplicationState(string, string, ApplicationState) error { return nil }
func (*MockStateStorage) SaveStackState(string, ApplicationState) error               { return nil }
func (*MockStateStorage) GetStackState(string) (map[string]ApplicationState, error) {
	return make(map[string]ApplicationState), nil
}

type MockTaskRunner struct {
	fillErr error
	runErr  error
}

func (m *MockTaskRunner) FillContext(context *StackContext, application *Application, task marathon.Task) error {
	context.SetStackVariable("foo", "bar")
	return m.fillErr
}
func (m *MockTaskRunner) RunTask(context *StackContext, application *Application, task map[string]string) error {
	return m.runErr
}

type FakeMesos struct{}

func (FakeMesos) Update() error           { return nil }
func (FakeMesos) GetActivatedSlaves() int { return 0 }
func (FakeMesos) GetSlaves() []Slave      { return nil }

type MockScheduler struct {
	startErr error
	state    MesosState
}

func (ms *MockScheduler) Start() error {
	return ms.startErr
}

func (ms *MockScheduler) RunApplication(app *Application) <-chan *ApplicationRunStatus {
	return nil
}

func (ms *MockScheduler) GetMesosState() MesosState {
	return ms.state
}

func (ms *MockScheduler) GetScheduledTasks() []*ScheduledTask {
	return []*ScheduledTask{}
}

func (ms *MockScheduler) RemoveScheduled(int64) bool {
	return true
}
