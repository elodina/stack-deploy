/* Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. */

package framework

import (
	"testing"

	"bytes"
	"errors"
	"github.com/gambol99/go-marathon"
	. "github.com/smartystreets/goconvey/convey"
	"gopkg.in/yaml.v2"
	"os"
	"time"
)

var validationCases map[*Application]error = map[*Application]error{
	&Application{
		Type:          "foo",
		ID:            "foo",
		Cpu:           0.5,
		Mem:           512,
		LaunchCommand: "sleep 10",
	}: nil,

	&Application{
		ID:            "notype",
		Cpu:           0.5,
		Mem:           512,
		LaunchCommand: "sleep 10",
	}: ErrApplicationNoType,

	&Application{
		Type:          "foo",
		Cpu:           0.5,
		Mem:           512,
		LaunchCommand: "sleep 10",
	}: ErrApplicationNoID,

	&Application{
		Type:          "foo",
		ID:            "nocpu",
		Mem:           512,
		LaunchCommand: "sleep 10",
	}: ErrApplicationInvalidCPU,

	&Application{
		Type:          "foo",
		ID:            "nomem",
		Cpu:           0.5,
		LaunchCommand: "sleep 10",
	}: ErrApplicationInvalidMem,

	&Application{
		Type: "foo",
		ID:   "nolaunchcmd",
		Cpu:  0.5,
		Mem:  512,
	}: ErrApplicationNoLaunchCommand,

	&Application{
		Type:          "foo",
		ID:            "invalidinstances",
		Cpu:           0.5,
		Mem:           512,
		Instances:     "-1",
		LaunchCommand: "sleep 10",
	}: ErrApplicationInvalidInstances,

	&Application{
		Type:          "foo",
		ID:            "invalidinstances",
		Cpu:           0.5,
		Mem:           512,
		Instances:     "bar",
		LaunchCommand: "sleep 10",
	}: ErrApplicationInvalidInstances,

	&Application{
		Type:          "bar",
		ID:            "invalidinstances",
		Cpu:           0.5,
		Mem:           512,
		LaunchCommand: "sleep 10",
		Tasks: yaml.MapSlice{
			yaml.MapItem{
				Key:   "foo",
				Value: "bar",
			},
		},
	}: ErrApplicationNoTaskRunner,
}

var dependencyPositiveCases map[*Application]map[string]ApplicationState = map[*Application]map[string]ApplicationState{
	&Application{
		Type:          "foo",
		ID:            "foo",
		Cpu:           0.5,
		Mem:           512,
		LaunchCommand: "sleep 10",
	}: map[string]ApplicationState{},

	&Application{
		Type:          "foo",
		ID:            "foo",
		Cpu:           0.5,
		Mem:           512,
		LaunchCommand: "sleep 10",
		Dependencies:  []string{"bar"},
	}: map[string]ApplicationState{
		"bar": StateRunning,
	},

	&Application{
		Type:          "foo",
		ID:            "foo",
		Cpu:           0.5,
		Mem:           512,
		LaunchCommand: "sleep 10",
		Dependencies:  []string{"bar", "baz"},
	}: map[string]ApplicationState{
		"bar": StateRunning,
		"baz": StateRunning,
		"bak": StateStaging,
		"bat": StateFail,
	},
}

var dependencyNegativeCases map[*Application]map[string]ApplicationState = map[*Application]map[string]ApplicationState{
	&Application{
		Type:          "foo",
		ID:            "foo",
		Cpu:           0.5,
		Mem:           512,
		LaunchCommand: "sleep 10",
		Dependencies:  []string{"bar"},
	}: map[string]ApplicationState{},

	&Application{
		Type:          "foo",
		ID:            "foo",
		Cpu:           0.5,
		Mem:           512,
		LaunchCommand: "sleep 10",
		Dependencies:  []string{"bar"},
	}: map[string]ApplicationState{
		"bar": StateStaging,
	},

	&Application{
		Type:          "foo",
		ID:            "foo",
		Cpu:           0.5,
		Mem:           512,
		LaunchCommand: "sleep 10",
		Dependencies:  []string{"bar"},
	}: map[string]ApplicationState{
		"bar": StateFail,
	},

	&Application{
		Type:          "foo",
		ID:            "foo",
		Cpu:           0.5,
		Mem:           512,
		LaunchCommand: "sleep 10",
		Dependencies:  []string{"bar", "baz"},
	}: map[string]ApplicationState{
		"bar": StateRunning,
		"baz": StateStaging,
	},
}

var ensureResolvedPositiveCases []interface{} = []interface{}{
	"./some_script.sh --debug",
	map[string]string{
		"foo": "bar",
		"asd": "zxc",
	},
	yaml.MapSlice{
		yaml.MapItem{
			Key:   "foo",
			Value: "bar",
		},
		yaml.MapItem{
			Key:   "asd",
			Value: "zxc",
		},
	},
}

var ensureResolvedNegativeCases []interface{} = []interface{}{
	"./some_script.sh --param ${foo}",
	map[string]string{
		"foo": "bar",
		"asd": "${foo}",
	},
	yaml.MapSlice{
		yaml.MapItem{
			Key:   "foo",
			Value: "${foo}",
		},
		yaml.MapItem{
			Key:   "asd",
			Value: "zxc",
		},
	},
}

func TestApplication(t *testing.T) {

	Convey("Validating applications", t, func() {
		TaskRunners = map[string]TaskRunner{
			"foo": new(MockTaskRunner),
		}

		Convey("Should fail for invalid cases", func() {
			for app, err := range validationCases {
				So(app.Validate(), ShouldEqual, err)
			}
		})

	})

	Convey("Application dependencies should resolve properly", t, func() {
		for app, state := range dependencyPositiveCases {
			So(app.IsDependencySatisfied(state), ShouldBeTrue)
		}

		for app, state := range dependencyNegativeCases {
			So(app.IsDependencySatisfied(state), ShouldBeFalse)
		}
	})

	Convey("Ensure variables resolved", t, func() {
		Convey("Should find unresolved variables", func() {
			for _, entry := range ensureResolvedPositiveCases {
				So(ensureVariablesResolved(nil, entry), ShouldBeNil)
			}

			for _, entry := range ensureResolvedNegativeCases {
				So(ensureVariablesResolved(nil, entry).Error(), ShouldContainSubstring, "Unresolved variable ${foo}")
			}
		})
	})

	Convey("Application should resolve variables", t, func() {
		ctx := NewContext()
		ctx.Set("foo", "bar")

		app := new(Application)
		app.LaunchCommand = "./${foo}.sh"
		app.Scheduler = map[string]string{
			"flag": "${foo}",
		}
		app.Tasks = yaml.MapSlice{
			yaml.MapItem{
				Key: "task",
				Value: yaml.MapSlice{
					yaml.MapItem{
						Key:   "param",
						Value: "${foo}",
					},
				},
			},
		}
		app.BeforeScheduler = []string{"${foo}"}
		app.AfterScheduler = []string{"${foo}"}
		app.BeforeTask = []string{"${foo}"}
		app.AfterTask = []string{"${foo}"}
		app.AfterTasks = []string{"${foo}"}

		app.resolveVariables(ctx)
		So(ensureVariablesResolved(ctx, app.LaunchCommand), ShouldBeNil)
		So(ensureVariablesResolved(ctx, app.Scheduler), ShouldBeNil)
		So(ensureVariablesResolved(ctx, app.Tasks), ShouldBeNil)
		So(ensureVariablesResolved(ctx, app.BeforeScheduler), ShouldBeNil)
		So(ensureVariablesResolved(ctx, app.AfterScheduler), ShouldBeNil)
		So(ensureVariablesResolved(ctx, app.BeforeTask), ShouldBeNil)
		So(ensureVariablesResolved(ctx, app.AfterTask), ShouldBeNil)
		So(ensureVariablesResolved(ctx, app.AfterTasks), ShouldBeNil)
	})

	Convey("Application should provide healthcheck if specified", t, func() {
		app := new(Application)
		So(app.getHealthchecks(), ShouldBeNil)

		app.Healthcheck = "/health"
		So(len(app.getHealthchecks()), ShouldEqual, 1)
	})

	Convey("Application should set the right number of instances", t, func() {
		Mesos = NewMesosState("")
		Mesos.ActivatedSlaves = 12
		app := new(Application)
		// 1 is default
		So(app.getInstances(), ShouldEqual, 1)

		app.Instances = "34"
		So(app.getInstances(), ShouldEqual, 34)

		app.Instances = "all"
		So(app.getInstances(), ShouldEqual, 12)

		app.Instances = "foo"
		So(func() { app.getInstances() }, ShouldPanic)
	})

	Convey("Application should form a correct launch string", t, func() {
		app := new(Application)
		app.LaunchCommand = "./script.sh"

		So(app.getLaunchCommand(), ShouldEqual, "./script.sh")

		app.Scheduler = map[string]string{
			"foo": "bar",
		}

		So(app.getLaunchCommand(), ShouldEqual, "./script.sh --foo bar")
	})

	Convey("Custom shell commands should run correctly", t, func() {
		buffer := new(bytes.Buffer)
		stdout = buffer
		defer func() {
			stdout = os.Stdout
		}()
		app := new(Application)

		So(app.executeCommands([]string{"echo stack-deploy"}, "__sd_test.sh"), ShouldBeNil)
		So(string(buffer.Bytes()), ShouldContainSubstring, "stack-deploy")
	})

	Convey("Application checks for running and healthy should work properly", t, func() {
		app := new(Application)
		app.ID = "foo"
		app.Healthcheck = "/health"

		client := NewMockMarathon()
		So(app.checkRunningAndHealthy(client), ShouldEqual, ErrApplicationDoesNotExist)

		client.applications["foo"] = new(marathon.Application)
		So(app.checkRunningAndHealthy(client), ShouldEqual, ErrTaskNotRunning)

		client.applications["foo"].TasksRunning = 1
		So(app.checkRunningAndHealthy(client), ShouldEqual, ErrHealthcheckNotPassing)

		client.applications["foo"].TasksHealthy = 1
		So(app.checkRunningAndHealthy(client), ShouldBeNil)

		client.err = errors.New("boom!")
		So(app.checkRunningAndHealthy(client).Error(), ShouldEqual, "boom!")
	})

	Convey("Await for application running and healthy", t, func() {
		app := new(Application)
		app.ID = "foo"
		app.Healthcheck = "/health"

		Convey("Should fail if time/retries exceeded", func() {
			client := NewMockMarathon()

			go func() {
				time.Sleep(500 * time.Millisecond)
				client.applications["foo"] = new(marathon.Application)
				client.applications["foo"].TasksRunning = 1
				client.applications["foo"].TasksHealthy = 1
			}()

			So(app.awaitRunningAndHealthy(client, 3, 100*time.Millisecond).Error(), ShouldContainSubstring, "Failed to await")
		})

		Convey("Should succeed if task becomes running and healthy in reasonable time", func() {
			client := NewMockMarathon()

			go func() {
				time.Sleep(200 * time.Millisecond)
				client.applications["foo"] = new(marathon.Application)
				client.applications["foo"].TasksRunning = 1
				client.applications["foo"].TasksHealthy = 1
			}()

			So(app.awaitRunningAndHealthy(client, 3, 100*time.Millisecond), ShouldBeNil)
		})
	})

	Convey("Task runner should fill application context properly", t, func() {
		ctx := NewContext()
		runner := new(MockTaskRunner)
		client := NewMockMarathon()

		app := new(Application)
		app.ID = "foo"

		client.tasks["foo"] = &marathon.Tasks{
			Tasks: []marathon.Task{
				marathon.Task{},
			},
		}

		So(len(ctx.All()), ShouldEqual, 0)
		So(app.fillContext(ctx, runner, client), ShouldBeNil)
		So(len(ctx.All()), ShouldEqual, 1)

		delete(client.tasks, "foo")
		So(app.fillContext(ctx, runner, client), ShouldEqual, ErrTaskNotRunning)

		client.err = errors.New("boom!")
		So(app.fillContext(ctx, runner, client).Error(), ShouldEqual, "boom!")
	})

}
