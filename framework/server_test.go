package framework

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
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
		marathonClient := NewMockMarathon()
		storage := &MockStorage{}
		userStorage := &MockUserStorage{}
		stateStorage := &MockStateStorage{}

		Convey("When creating new API Server", func() {
			server := NewApiServer(api, marathonClient, nil, storage, userStorage, stateStorage)

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

		Convey("Stack management", func() {
			Convey("/list", func() {
				req, _ := http.NewRequest("GET", TestEndpoint+"/list", nil)
				req.Header.Add("X-Api-User", "user")
				req.Header.Add("X-Api-Key", "key")
				client := &http.Client{}
				resp, err := client.Do(req)
				Convey("It should not return error", func() {
					So(err, ShouldBeNil)
				})
				Convey("It should return status code 200 OK", func() {
					So(resp.StatusCode, ShouldEqual, 200)
				})
				Convey("It should return list of stacks", func() {
					bytes, err := ioutil.ReadAll(resp.Body)
					So(err, ShouldBeNil)
					content := string(bytes)
					So(content, ShouldNotBeEmpty)
					So(content, ShouldContainSubstring, "stack1")
				})

			})

			Convey("/get", func() {
				Convey("With right name", func() {
					stack := map[string]string{"name": "stack1"}
					encoded, _ := json.Marshal(stack)
					reader := bytes.NewReader(encoded)
					req, _ := http.NewRequest("POST", TestEndpoint+"/get", reader)
					req.Header.Add("X-Api-User", "user")
					req.Header.Add("X-Api-Key", "key")
					client := &http.Client{}
					resp, err := client.Do(req)
					Convey("It should not return error", func() {
						So(err, ShouldBeNil)
					})
					Convey("It should return status code 200 OK", func() {
						So(resp.StatusCode, ShouldEqual, 200)
					})
					Convey("It should return requested stack", func() {
						bytes, err := ioutil.ReadAll(resp.Body)
						So(err, ShouldBeNil)
						content := string(bytes)
						So(content, ShouldNotBeEmpty)
						So(content, ShouldContainSubstring, "stack1")
					})
				})
				Convey("With wrong name", func() {
					stack := map[string]string{"name": "stack2"}
					encoded, _ := json.Marshal(stack)
					reader := bytes.NewReader(encoded)
					req, _ := http.NewRequest("POST", TestEndpoint+"/get", reader)
					req.Header.Add("X-Api-User", "user")
					req.Header.Add("X-Api-Key", "key")
					client := &http.Client{}
					resp, err := client.Do(req)
					Convey("It should not return error", func() {
						So(err, ShouldBeNil)
					})
					Convey("It should return status code 404 Not Found", func() {
						So(resp.StatusCode, ShouldEqual, 404)
					})
					Convey("It should return no stack", func() {
						bytes, err := ioutil.ReadAll(resp.Body)
						So(err, ShouldBeNil)
						content := string(bytes)
						So(content, ShouldNotBeEmpty)
						So(content, ShouldNotContainSubstring, "stack1")
					})
				})

			})

			Convey("/run", func() {
				Convey("Without zone", func() {
					Mesos = &FakeMesos{}
					stack := map[string]string{"name": "stack1", "zone": ""}
					encoded, _ := json.Marshal(stack)
					reader := bytes.NewReader(encoded)
					req, _ := http.NewRequest("POST", TestEndpoint+"/run", reader)
					req.Header.Add("X-Api-User", "user")
					req.Header.Add("X-Api-Key", "key")
					client := &http.Client{}
					resp, err := client.Do(req)
					Convey("It should not return error", func() {
						So(err, ShouldBeNil)
					})
					Convey("It should return status code 200 OK", func() {
						if resp.StatusCode != 200 {
							content, _ := ioutil.ReadAll(resp.Body)
							fmt.Println(string(content))
						}
						So(resp.StatusCode, ShouldEqual, 200)
					})
					Convey("It should return empty response", func() {
						bytes, err := ioutil.ReadAll(resp.Body)
						So(err, ShouldBeNil)
						content := string(bytes)
						So(content, ShouldBeEmpty)
					})
				})
				Convey("With zone", func() {
					Mesos = &FakeMesos{}
					stack := map[string]string{"name": "stack1", "zone": "default"}
					encoded, _ := json.Marshal(stack)
					reader := bytes.NewReader(encoded)
					req, _ := http.NewRequest("POST", TestEndpoint+"/run", reader)
					req.Header.Add("X-Api-User", "user")
					req.Header.Add("X-Api-Key", "key")
					client := &http.Client{}
					resp, err := client.Do(req)
					Convey("It should not return error", func() {
						So(err, ShouldBeNil)
					})
					Convey("It should return status code 200 OK", func() {
						if resp.StatusCode != 200 {
							content, _ := ioutil.ReadAll(resp.Body)
							fmt.Println(string(content))
						}
						So(resp.StatusCode, ShouldEqual, 200)
					})
					Convey("It should return empty response", func() {
						bytes, err := ioutil.ReadAll(resp.Body)
						So(err, ShouldBeNil)
						content := string(bytes)
						So(content, ShouldBeEmpty)
					})
				})

			})

			Convey("/createstack", func() {
				request := map[string]string{"stackfile": "name: test\napplications:\n"}
				encoded, _ := json.Marshal(request)
				reader := bytes.NewReader(encoded)
				req, _ := http.NewRequest("POST", TestEndpoint+"/createstack", reader)
				req.Header.Add("X-Api-User", "user")
				req.Header.Add("X-Api-Key", "key")
				client := &http.Client{}
				resp, err := client.Do(req)
				Convey("It should not return error", func() {
					So(err, ShouldBeNil)
				})
				Convey("It should return status code 200 OK", func() {
					if resp.StatusCode != 200 {
						content, _ := ioutil.ReadAll(resp.Body)
						fmt.Println(string(content))
					}
					So(resp.StatusCode, ShouldEqual, 200)
				})
			})

			Convey("/removestack", func() {
				stack := map[string]string{"name": "stack1", "force": "true"}
				encoded, _ := json.Marshal(stack)
				reader := bytes.NewReader(encoded)
				req, _ := http.NewRequest("POST", TestEndpoint+"/removestack", reader)
				req.Header.Add("X-Api-User", "user")
				req.Header.Add("X-Api-Key", "key")
				client := &http.Client{}
				resp, err := client.Do(req)
				Convey("It should not return error", func() {
					So(err, ShouldBeNil)
				})
				Convey("It should return status code 200 OK", func() {
					if resp.StatusCode != 200 {
						content, _ := ioutil.ReadAll(resp.Body)
						fmt.Println(string(content))
					}
					So(resp.StatusCode, ShouldEqual, 200)
				})
			})

			Convey("/createlayer", func() {
				Convey("Create zone", func() {
					resp, err := createLayer("zone")
					Convey("It should not return error", func() {
						So(err, ShouldBeNil)
					})
					Convey("It should return status code 200 OK", func() {
						if resp.StatusCode != 200 {
							content, _ := ioutil.ReadAll(resp.Body)
							fmt.Println(string(content))
						}
						So(resp.StatusCode, ShouldEqual, 200)
					})
				})
				Convey("Create cluster", func() {
					resp, err := createLayer("cluster")
					Convey("It should not return error", func() {
						So(err, ShouldBeNil)
					})
					Convey("It should return status code 200 OK", func() {
						if resp.StatusCode != 200 {
							content, _ := ioutil.ReadAll(resp.Body)
							fmt.Println(string(content))
						}
						So(resp.StatusCode, ShouldEqual, 200)
					})
				})
				Convey("Create datacenter", func() {
					resp, err := createLayer("datacenter")
					Convey("It should not return error", func() {
						So(err, ShouldBeNil)
					})
					Convey("It should return status code 200 OK", func() {
						if resp.StatusCode != 200 {
							content, _ := ioutil.ReadAll(resp.Body)
							fmt.Println(string(content))
						}
						So(resp.StatusCode, ShouldEqual, 200)
					})
				})
				Convey("Create invalid zone", func() {
					resp, err := createLayer("invalid")
					Convey("It should not return error", func() {
						So(err, ShouldBeNil)
					})
					Convey("It should return status code 400 Invalid Request", func() {
						So(resp.StatusCode, ShouldEqual, 400)
					})
				})

			})
		})

	})

}

func createLayer(layer string) (*http.Response, error) {
	request := map[string]string{"stackfile": "name: test\napplications:\n", "layer": layer}
	encoded, _ := json.Marshal(request)
	reader := bytes.NewReader(encoded)
	req, _ := http.NewRequest("POST", TestEndpoint+"/createlayer", reader)
	req.Header.Add("X-Api-User", "user")
	req.Header.Add("X-Api-Key", "key")
	client := &http.Client{}
	return client.Do(req)
}
