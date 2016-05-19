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
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestInMemoryStateStorage(t *testing.T) {
	Convey("In memory state storage should", t, func() {
		Convey("save stack status", func() {
			storage := NewInMemoryStateStorage()
			So(storage.stackStates, ShouldBeEmpty)

			err := storage.SaveStackStatus("foo", "", StackStatusRunning)
			So(err, ShouldBeNil)
			So(storage.stackStates, ShouldHaveLength, 1)
			So(storage.stackStates["foo"], ShouldHaveLength, 1)
			So(storage.stackStates["foo"][""].Status, ShouldEqual, StackStatusRunning)
		})

		Convey("fail to save duplicate stack state", func() {
			storage := NewInMemoryStateStorage()
			err := storage.SaveStackStatus("foo", "", StackStatusRunning)
			So(err, ShouldBeNil)

			err = storage.SaveStackStatus("foo", "", StackStatusRunning)
			So(err, ShouldEqual, ErrStackStateExists)
		})

		Convey("save application status", func() {
			storage := NewInMemoryStateStorage()
			err := storage.SaveStackStatus("foo", "", StackStatusRunning)
			So(err, ShouldBeNil)

			err = storage.SaveApplicationStatus("foo", "", "bar", ApplicationStatusRunning)
			So(err, ShouldBeNil)
			So(storage.stackStates["foo"][""].Applications["bar"], ShouldEqual, ApplicationStatusRunning)
		})

		Convey("fail to save application status for inexisting stack name", func() {
			storage := NewInMemoryStateStorage()
			err := storage.SaveApplicationStatus("foo", "", "bar", ApplicationStatusRunning)
			So(err, ShouldEqual, ErrStackStateDoesNotExist)
		})

		Convey("fail to save application status for inexisting zone", func() {
			storage := NewInMemoryStateStorage()
			err := storage.SaveStackStatus("foo", "", StackStatusRunning)
			So(err, ShouldBeNil)

			err = storage.SaveApplicationStatus("foo", "bar", "baz", ApplicationStatusRunning)
			So(err, ShouldEqual, ErrStackStateDoesNotExist)
		})

		Convey("save stack variables", func() {
			storage := NewInMemoryStateStorage()
			err := storage.SaveStackStatus("foo", "", StackStatusRunning)
			So(err, ShouldBeNil)

			variables := NewVariables()
			variables.SetStackVariable("foo", "bar")
			err = storage.SaveStackVariables("foo", "", variables)
			So(err, ShouldBeNil)
		})

		Convey("fail to save stack variables for inexisting stack name", func() {
			storage := NewInMemoryStateStorage()
			err := storage.SaveStackVariables("foo", "", NewVariables())
			So(err, ShouldEqual, ErrStackStateDoesNotExist)
		})

		Convey("fail to save stack variables for inexisting zone", func() {
			storage := NewInMemoryStateStorage()
			err := storage.SaveStackStatus("foo", "", StackStatusRunning)
			So(err, ShouldBeNil)

			err = storage.SaveStackVariables("foo", "bar", NewVariables())
			So(err, ShouldEqual, ErrStackStateDoesNotExist)
		})

		Convey("get stack state", func() {
			storage := NewInMemoryStateStorage()
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
			storage := NewInMemoryStateStorage()
			state, err := storage.GetStackState("foo", "zone")
			So(err, ShouldEqual, ErrStackStateDoesNotExist)
			So(state, ShouldBeNil)
		})

		Convey("fail to get stack state for inexisting zone", func() {
			storage := NewInMemoryStateStorage()
			err := storage.SaveStackStatus("foo", "", StackStatusRunning)
			So(err, ShouldBeNil)

			state, err := storage.GetStackState("foo", "bar")
			So(err, ShouldEqual, ErrStackStateDoesNotExist)
			So(state, ShouldBeNil)
		})
	})
}
