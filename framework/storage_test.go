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
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"os"
	"testing"
	"time"
)

var testCassandraConnect = []string{"localhost:9042"}
var executionTime = time.Now().Unix()

func TestCassandraStorage(t *testing.T) {
	Convey("Cassandra storage should", t, func() {
		storage, _, err := NewCassandraStorage(testCassandraConnect, fmt.Sprintf("stackdeploytest%d", executionTime), 3)
		if err != nil {
			if os.Getenv("TRAVIS_CI") != "" {
				t.Fatal(err.Error())
			} else {
				t.Skip("Cannot connect to Cassandra. Please spin up Cassandra at localhost:9042 for this test to run.")
			}
		}

		Convey("add stacks properly", func() {
			err = storage.StoreStack(&Stack{
				Name: "foo",
			})
			So(err, ShouldBeNil)

			err = storage.StoreStack(&Stack{
				Name: "foo",
			})

			So(err, ShouldNotBeNil)
			So(err, ShouldEqual, ErrStackExists)
		})

		Convey("remove existing stack", func() {
			err := storage.RemoveStack("foo", false)
			So(err, ShouldBeNil)
		})

		Convey("remove stack", func() {
			Convey("errors when stack does not exist", func() {
				err := storage.RemoveStack("foo", false)
				So(err, ShouldNotBeNil)
				So(err, ShouldEqual, ErrStackDoesNotExist)

				err = storage.RemoveStack("foo", true)
				So(err, ShouldNotBeNil)
				So(err, ShouldEqual, ErrStackDoesNotExist)
			})

			Convey("remove stack when no other stacks depend on it", func() {
				err := storage.StoreStack(&Stack{
					Name: "foo",
				})
				So(err, ShouldBeNil)

				stacks, err := storage.GetAll()
				So(err, ShouldBeNil)
				So(stacks, ShouldHaveLength, 1)

				err = storage.RemoveStack("foo", false)
				So(err, ShouldBeNil)

				stacks, err = storage.GetAll()
				So(err, ShouldBeNil)
				So(stacks, ShouldBeEmpty)
			})

			Convey("fail to remove a stack when there are dependant stacks and force == false", func() {
				err := storage.StoreStack(&Stack{
					Name: "foo",
				})
				So(err, ShouldBeNil)

				err = storage.StoreStack(&Stack{
					Name: "foo2",
					From: "foo",
				})
				So(err, ShouldBeNil)

				err = storage.RemoveStack("foo", false)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "There are stacks depending")
			})

			Convey("remove a stack and all its children with force == true", func() {
				stacks, err := storage.GetAll()
				So(err, ShouldBeNil)
				So(stacks, ShouldHaveLength, 2)

				err = storage.RemoveStack("foo", true)
				So(err, ShouldBeNil)

				stacks, err = storage.GetAll()
				So(err, ShouldBeNil)
				So(stacks, ShouldBeEmpty)
			})
		})

		Convey("get all stacks", func() {
			stacks, err := storage.GetAll()
			So(err, ShouldBeNil)
			So(stacks, ShouldBeEmpty)

			err = storage.StoreStack(&Stack{
				Name: "foo",
			})
			So(err, ShouldBeNil)

			stacks, err = storage.GetAll()
			So(err, ShouldBeNil)
			So(stacks, ShouldHaveLength, 1)

			err = storage.RemoveStack("foo", false)
			So(err, ShouldBeNil)
		})

		Convey("get one stack", func() {
			stack, err := storage.GetStack("foo")
			So(stack, ShouldBeNil)
			So(err, ShouldNotBeNil)
			So(err, ShouldEqual, ErrStackDoesNotExist)

			runner, err := storage.GetStackRunner("foo")
			So(runner, ShouldBeNil)
			So(err, ShouldNotBeNil)
			So(err, ShouldEqual, ErrStackDoesNotExist)

			err = storage.StoreStack(&Stack{
				Name: "foo",
			})
			So(err, ShouldBeNil)

			stack, err = storage.GetStack("foo")
			So(err, ShouldBeNil)
			So(stack, ShouldNotBeNil)
			So(stack.Name, ShouldEqual, "foo")

			runner, err = storage.GetStackRunner("foo")
			So(err, ShouldBeNil)
			So(runner, ShouldNotBeNil)

			err = storage.RemoveStack("foo", false)
			So(err, ShouldBeNil)
		})

		Convey("get stack tree", func() {
			err = storage.StoreStack(&Stack{
				Name: "foo",
			})
			So(err, ShouldBeNil)

			err = storage.StoreStack(&Stack{
				From: "foo",
				Name: "bar",
			})
			So(err, ShouldBeNil)

			stack, err := storage.GetStack("bar")
			So(err, ShouldBeNil)
			So(stack, ShouldNotBeNil)
			So(stack.Name, ShouldEqual, "bar")
		})

		Convey("save stack status", func() {
			err := storage.SaveStackStatus("foo", "", StackStatusRunning)
			So(err, ShouldBeNil)
		})

		Convey("save application status", func() {
			err = storage.SaveApplicationStatus("foo", "", "bar", ApplicationStatusRunning)
			So(err, ShouldBeNil)
		})

		Convey("save stack variables", func() {
			variables := NewVariables()
			variables.SetStackVariable("foo", "bar")
			err = storage.SaveStackVariables("foo", "", variables)
			So(err, ShouldBeNil)
		})

		Convey("get stack state", func() {
			err := storage.SaveStackStatus("foo", "zone", StackStatusRunning)
			So(err, ShouldBeNil)

			err = storage.SaveApplicationStatus("foo", "zone", "bar", ApplicationStatusRunning)
			So(err, ShouldBeNil)

			variables := NewVariables()
			variables.SetStackVariable("foo", "bar")
			err = storage.SaveStackVariables("foo", "zone", variables)
			So(err, ShouldBeNil)

			state, err := storage.GetStackState("foo", "zone")
			So(err, ShouldBeNil)
			So(state, ShouldNotBeNil)
			So(state.Name, ShouldEqual, "foo")
			So(state.Zone, ShouldEqual, "zone")
			So(state.Applications["bar"], ShouldEqual, ApplicationStatusRunning)
			So(state.Status, ShouldEqual, StackStatusRunning)
			So(state.Variables.stackVariables["foo"], ShouldEqual, "bar")
		})

		Convey("fail to get stack state for inexisting stack name", func() {
			state, err := storage.GetStackState("foobar", "zone")
			So(err, ShouldEqual, ErrStackStateDoesNotExist)
			So(state, ShouldBeNil)
		})

		Convey("fail to get stack state for inexisting zone", func() {
			err := storage.SaveStackStatus("foobar", "", StackStatusRunning)
			So(err, ShouldBeNil)

			state, err := storage.GetStackState("foobar", "somezone")
			So(err, ShouldEqual, ErrStackStateDoesNotExist)
			So(state, ShouldBeNil)
		})
	})
}

func TestInMemoryStorage(t *testing.T) {
	Convey("In memory storage should", t, func() {
		Convey("add stacks properly", func() {
			storage := NewInMemoryStorage()
			So(storage.stacks, ShouldBeEmpty)
			err := storage.StoreStack(&Stack{
				Name: "foo",
			})

			So(err, ShouldBeNil)
			So(storage.stacks, ShouldHaveLength, 1)

			err = storage.StoreStack(&Stack{
				Name: "foo",
			})

			So(err, ShouldNotBeNil)
			So(err, ShouldEqual, ErrStackExists)
			So(storage.stacks, ShouldHaveLength, 1)
		})

		Convey("get all stacks", func() {
			storage := NewInMemoryStorage()
			stacks, err := storage.GetAll()
			So(err, ShouldBeNil)
			So(stacks, ShouldBeEmpty)

			err = storage.StoreStack(&Stack{
				Name: "foo",
			})
			So(err, ShouldBeNil)

			stacks, err = storage.GetAll()
			So(err, ShouldBeNil)
			So(stacks, ShouldHaveLength, 1)
		})

		Convey("get one stack", func() {
			storage := NewInMemoryStorage()
			stack, err := storage.GetStack("foo")
			So(stack, ShouldBeNil)
			So(err, ShouldNotBeNil)
			So(err, ShouldEqual, ErrStackDoesNotExist)

			runner, err := storage.GetStackRunner("foo")
			So(runner, ShouldBeNil)
			So(err, ShouldNotBeNil)
			So(err, ShouldEqual, ErrStackDoesNotExist)

			err = storage.StoreStack(&Stack{
				Name: "foo",
			})
			So(err, ShouldBeNil)

			stack, err = storage.GetStack("foo")
			So(err, ShouldBeNil)
			So(stack, ShouldNotBeNil)
			So(stack.Name, ShouldEqual, "foo")

			runner, err = storage.GetStackRunner("foo")
			So(err, ShouldBeNil)
			So(runner, ShouldNotBeNil)
		})

		Convey("remove stack", func() {
			storage := NewInMemoryStorage()

			Convey("errors when stack does not exist", func() {
				err := storage.RemoveStack("foo", false)
				So(err, ShouldNotBeNil)
				So(err, ShouldEqual, ErrStackDoesNotExist)

				err = storage.RemoveStack("foo", true)
				So(err, ShouldNotBeNil)
				So(err, ShouldEqual, ErrStackDoesNotExist)
			})

			Convey("remove stack when no other stacks depend on it", func() {
				err := storage.StoreStack(&Stack{
					Name: "foo",
				})
				So(err, ShouldBeNil)
				So(storage.stacks, ShouldHaveLength, 1)

				err = storage.RemoveStack("foo", false)
				So(err, ShouldBeNil)
				So(storage.stacks, ShouldBeEmpty)
			})

			Convey("fail to remove a stack when there are dependant stacks and force == false", func() {
				err := storage.StoreStack(&Stack{
					Name: "foo",
				})
				So(err, ShouldBeNil)

				err = storage.StoreStack(&Stack{
					Name: "foo2",
					From: "foo",
				})
				So(err, ShouldBeNil)

				err = storage.RemoveStack("foo", false)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldContainSubstring, "There are stacks depending")
			})

			Convey("remove a stack and all its children with force == true", func() {
				err := storage.StoreStack(&Stack{
					Name: "foo",
				})
				So(err, ShouldBeNil)

				err = storage.StoreStack(&Stack{
					Name: "foo2",
					From: "foo",
				})
				So(err, ShouldBeNil)
				So(storage.stacks, ShouldHaveLength, 2)

				err = storage.RemoveStack("foo", true)
				So(err, ShouldBeNil)
				So(storage.stacks, ShouldBeEmpty)
			})
		})

		Convey("save stack status", func() {
			storage := NewInMemoryStorage()
			So(storage.stackStates, ShouldBeEmpty)

			err := storage.SaveStackStatus("foo", "", StackStatusRunning)
			So(err, ShouldBeNil)
			So(storage.stackStates, ShouldHaveLength, 1)
			So(storage.stackStates["foo"], ShouldHaveLength, 1)
			So(storage.stackStates["foo"][""].Status, ShouldEqual, StackStatusRunning)
		})

		Convey("save application status", func() {
			storage := NewInMemoryStorage()
			err := storage.SaveStackStatus("foo", "", StackStatusRunning)
			So(err, ShouldBeNil)

			err = storage.SaveApplicationStatus("foo", "", "bar", ApplicationStatusRunning)
			So(err, ShouldBeNil)
			So(storage.stackStates["foo"][""].Applications["bar"], ShouldEqual, ApplicationStatusRunning)
		})

		Convey("fail to save application status for inexisting stack name", func() {
			storage := NewInMemoryStorage()
			err := storage.SaveApplicationStatus("foo", "", "bar", ApplicationStatusRunning)
			So(err, ShouldEqual, ErrStackStateDoesNotExist)
		})

		Convey("fail to save application status for inexisting zone", func() {
			storage := NewInMemoryStorage()
			err := storage.SaveStackStatus("foo", "", StackStatusRunning)
			So(err, ShouldBeNil)

			err = storage.SaveApplicationStatus("foo", "bar", "baz", ApplicationStatusRunning)
			So(err, ShouldEqual, ErrStackStateDoesNotExist)
		})

		Convey("save stack variables", func() {
			storage := NewInMemoryStorage()
			err := storage.SaveStackStatus("foo", "", StackStatusRunning)
			So(err, ShouldBeNil)

			variables := NewVariables()
			variables.SetStackVariable("foo", "bar")
			err = storage.SaveStackVariables("foo", "", variables)
			So(err, ShouldBeNil)
		})

		Convey("fail to save stack variables for inexisting stack name", func() {
			storage := NewInMemoryStorage()
			err := storage.SaveStackVariables("foo", "", NewVariables())
			So(err, ShouldEqual, ErrStackStateDoesNotExist)
		})

		Convey("fail to save stack variables for inexisting zone", func() {
			storage := NewInMemoryStorage()
			err := storage.SaveStackStatus("foo", "", StackStatusRunning)
			So(err, ShouldBeNil)

			err = storage.SaveStackVariables("foo", "bar", NewVariables())
			So(err, ShouldEqual, ErrStackStateDoesNotExist)
		})

		Convey("get stack state", func() {
			storage := NewInMemoryStorage()
			err := storage.SaveStackStatus("foo", "zone", StackStatusRunning)
			So(err, ShouldBeNil)

			err = storage.SaveApplicationStatus("foo", "zone", "bar", ApplicationStatusRunning)
			So(err, ShouldBeNil)

			variables := NewVariables()
			variables.SetStackVariable("foo", "bar")
			err = storage.SaveStackVariables("foo", "zone", variables)
			So(err, ShouldBeNil)

			state, err := storage.GetStackState("foo", "zone")
			So(err, ShouldBeNil)
			So(state, ShouldNotBeNil)
			So(state.Name, ShouldEqual, "foo")
			So(state.Zone, ShouldEqual, "zone")
			So(state.Applications["bar"], ShouldEqual, ApplicationStatusRunning)
			So(state.Status, ShouldEqual, StackStatusRunning)
			So(state.Variables.stackVariables["foo"], ShouldEqual, "bar")
		})

		Convey("fail to get stack state for inexisting stack name", func() {
			storage := NewInMemoryStorage()
			state, err := storage.GetStackState("foo", "zone")
			So(err, ShouldEqual, ErrStackStateDoesNotExist)
			So(state, ShouldBeNil)
		})

		Convey("fail to get stack state for inexisting zone", func() {
			storage := NewInMemoryStorage()
			err := storage.SaveStackStatus("foo", "", StackStatusRunning)
			So(err, ShouldBeNil)

			state, err := storage.GetStackState("foo", "bar")
			So(err, ShouldEqual, ErrStackStateDoesNotExist)
			So(state, ShouldBeNil)
		})
	})
}
