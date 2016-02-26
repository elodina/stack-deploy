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

func TestInMemoryStorage(t *testing.T) {
	Convey("In memory storage", t, func() {
		Convey("should add stacks properly", func() {
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
			So(err.Error(), ShouldEqual, "Stack already exists")
			So(storage.stacks, ShouldHaveLength, 1)
		})

		Convey("should get all stacks", func() {
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

		Convey("should get one stack", func() {
			storage := NewInMemoryStorage()
			stack, err := storage.GetStack("foo")
			So(stack, ShouldBeNil)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldEqual, "Stack does not exist")

			runner, err := storage.GetStackRunner("foo")
			So(runner, ShouldBeNil)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldEqual, "Stack does not exist")

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

		Convey("should remove stack", func() {
			storage := NewInMemoryStorage()

			Convey("errors when stack does not exist", func() {
				err := storage.RemoveStack("foo", false)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldEqual, "Stack does not exist")

				err = storage.RemoveStack("foo", true)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldEqual, "Stack does not exist")
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

		Convey("should not error on init", func() {
			storage := NewInMemoryStorage()
			So(storage.Init(), ShouldBeNil)
		})
	})
}
