package framework

import marathon "github.com/gambol99/go-marathon"

type MockStorage struct{}

func (*MockStorage) GetAll() ([]*Stack, error)             { return make([]*Stack, 0), nil }
func (*MockStorage) GetStack(string) (*Stack, error)       { return nil, nil }
func (*MockStorage) StoreStack(*Stack) error               { return nil }
func (*MockStorage) RemoveStack(string, bool) error        { return nil }
func (*MockStorage) Init() error                           { return nil }
func (*MockStorage) GetLayersStack(string) (*Stack, error) { return nil, nil }

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

type MockTaskRunner struct{}

func (*MockTaskRunner) FillContext(context *Context, application *Application, task marathon.Task) error {
	return nil
}
func (*MockTaskRunner) RunTask(context *Context, application *Application, task map[string]string) error {
	return nil
}
