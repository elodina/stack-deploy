package framework

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

const (
	TestEndpoint = "http://127.0.0.1:1308"
)

func TestNewApiServer(t *testing.T) {

	Convey("Given an API endpoint, marathon client and storages", t, func() {
		api := TestEndpoint
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

			Convey("When starting API server", func() {
				Convey("It should not panic", func() {
					So(func() { go server.Start(); time.Sleep(10 * time.Millisecond) }, ShouldNotPanic)
				})
			})

			Convey("When starting second API server", func() {
				Convey("It should panic", func() {
					So(func() { server.Start() }, ShouldPanic)
				})
			})

		})

	})

}

func TestHandlers(t *testing.T) {

	Convey("Given an API server started", t, func() {

		Convey("When getting /health endpoint", func() {
			resp, err := http.Get(TestEndpoint + "/health")
			Convey("It should not return error", func() {
				So(err, ShouldBeNil)
			})
			Convey("It should return status code 200", func() {
				So(resp.StatusCode, ShouldEqual, 200)
			})
		})

		Convey("Require auth", func() {
			urls := []string{"/list",
				"/get",
				"/run",
				"/createstack",
				"/removestack",
				"/createuser",
				"/refreshtoken",
				"/createlayer",
			}
			for _, url := range urls {
				resp, _ := http.Post(TestEndpoint+url, "application/json", bytes.NewReader([]byte{}))
				Convey(url+" should require credentials", func() {
					So(resp.StatusCode, ShouldEqual, 403)
				})
			}
		})

		Convey("Require admin auth", func() {
			urls := []string{"/createuser", "/refreshtoken"}
			for _, url := range urls {
				req, _ := http.NewRequest("POST", TestEndpoint+"/createuser", bytes.NewReader([]byte{}))
				req.Header.Add("X-Api-User", "notadmin")
				req.Header.Add("X-Api-Key", "key")
				client := &http.Client{}
				resp, _ := client.Do(req)
				Convey(url+" should require admin credentials", func() {
					So(resp.StatusCode, ShouldEqual, 403)
				})
			}
		})

		Convey("User management", func() {
			Convey("/createuser", func() {
				user := map[string]string{"name": "test", "role": "admin"}
				encoded, _ := json.Marshal(user)
				reader := bytes.NewReader(encoded)
				req, _ := http.NewRequest("POST", TestEndpoint+"/createuser", reader)
				req.Header.Add("X-Api-User", "admin")
				req.Header.Add("X-Api-Key", "key")
				client := &http.Client{}
				resp, err := client.Do(req)
				Convey("It should not return error", func() {
					So(err, ShouldBeNil)
				})
				Convey("It should return status code 201 Created", func() {
					So(resp.StatusCode, ShouldEqual, 201)
				})
			})

			Convey("/refreshtoken", func() {
				user := map[string]string{"name": "test"}
				encoded, _ := json.Marshal(user)
				reader := bytes.NewReader(encoded)
				req, _ := http.NewRequest("POST", TestEndpoint+"/refreshtoken", reader)
				req.Header.Add("X-Api-User", "admin")
				req.Header.Add("X-Api-Key", "key")
				client := &http.Client{}
				resp, err := client.Do(req)
				Convey("It should not return error", func() {
					So(err, ShouldBeNil)
				})
				Convey("It should return status code 200 OK", func() {
					So(resp.StatusCode, ShouldEqual, 200)
				})
			})
		})

	})

}
