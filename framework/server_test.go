package framework

import (
	marathon "github.com/gambol99/go-marathon"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestNewApiServer(t *testing.T) {

	Convey("Given an API endpoint, marathon client and storages", t, func() {
		api := "http://127.0.0.1:1308"
		marathonClient := MockMarathon("127.0.0.1:8080")
		storage := &MockStorage{}
		userStorage := &MockUserStorage{}
		stateStorage := &MockStateStorage{}

		Convey("When creating new API Server", func() {
			server := NewApiServer(api, marathonClient, storage, userStorage, stateStorage)

			Convey("It should return not nil server", func() {
				So(server, ShouldNotBeNil)
			})

			Convey("Strips http:// prefix from url", func() {
				So(server.api, ShouldNotContainSubstring, "http://")
			})

		})

	})

}

type MockStorage struct{}

func (*MockStorage) GetAll() ([]*Stack, error)             { return make([]*Stack, 0), nil }
func (*MockStorage) GetStack(string) (*Stack, error)       { return nil, nil }
func (*MockStorage) StoreStack(*Stack) error               { return nil }
func (*MockStorage) RemoveStack(string, bool) error        { return nil }
func (*MockStorage) Init() error                           { return nil }
func (*MockStorage) GetLayersStack(string) (*Stack, error) { return nil, nil }

type MockUserStorage struct{}

func (*MockUserStorage) SaveUser(User) error                         { return nil }
func (*MockUserStorage) GetUser(string) (*User, error)               { return nil, nil }
func (*MockUserStorage) CheckKey(string, string) (bool, error)       { return false, nil }
func (*MockUserStorage) IsAdmin(string) (bool, error)                { return false, nil }
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

func MockMarathon(url string) marathon.Marathon {
	marathonConfig := marathon.NewDefaultConfig()
	marathonConfig.URL = url
	marathonClient, _ := marathon.NewClient(marathonConfig)
	return marathonClient
}
