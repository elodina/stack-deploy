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

package constraints

import (
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestConstraintParse(t *testing.T) {
	Convey("Like constraint", t, func() {
		Convey("Should parse on valid input", func() {
			constraint, err := ParseConstraint([]string{"LIKE", "1"})
			So(err, ShouldBeNil)
			like, ok := constraint.(*Like)
			So(ok, ShouldBeTrue)
			So(like.regex, ShouldEqual, "1")
		})
	})

	Convey("Unlike constraint", t, func() {
		Convey("Should parse on valid input", func() {
			constraint, err := ParseConstraint([]string{"UNLIKE", "1"})
			So(err, ShouldBeNil)
			unlike, ok := constraint.(*Unlike)
			So(ok, ShouldBeTrue)
			So(unlike.regex, ShouldEqual, "1")
		})
	})

	Convey("Unique constraint", t, func() {
		Convey("Should parse on valid input", func() {
			constraint, err := ParseConstraint([]string{"UNIQUE"})
			So(err, ShouldBeNil)
			_, ok := constraint.(*Unique)
			So(ok, ShouldBeTrue)
		})
	})

	Convey("Cluster constraint", t, func() {
		Convey("Should parse on empty condition", func() {
			constraint, err := ParseConstraint([]string{"CLUSTER"})
			So(err, ShouldBeNil)
			cluster, ok := constraint.(*Cluster)
			So(ok, ShouldBeTrue)
			So(cluster.value, ShouldBeEmpty)
		})

		Convey("Should parse on non-empty condition", func() {
			constraint, err := ParseConstraint([]string{"CLUSTER", "123"})
			So(err, ShouldBeNil)
			cluster123, ok := constraint.(*Cluster)
			So(ok, ShouldBeTrue)
			So(cluster123.value, ShouldEqual, "123")
		})
	})

	Convey("GroupBy constraint", t, func() {
		Convey("Should parse on empty condition", func() {
			constraint, err := ParseConstraint([]string{"GROUP_BY"})
			So(err, ShouldBeNil)
			groupBy, ok := constraint.(*GroupBy)
			So(ok, ShouldBeTrue)
			So(groupBy.groups, ShouldEqual, 1)
		})

		Convey("Should parse on non-empty condition", func() {
			constraint, err := ParseConstraint([]string{"GROUP_BY", "3"})
			So(err, ShouldBeNil)
			groupBy3, ok := constraint.(*GroupBy)
			So(ok, ShouldBeTrue)
			So(groupBy3.groups, ShouldEqual, 3)
		})
	})

	Convey("Invalid constraint", t, func() {
		Convey("Should not parse", func() {
			constraint, err := ParseConstraint([]string{"unsupported"})
			So(err, ShouldNotBeNil)
			So(constraint, ShouldBeNil)
			So(err.Error(), ShouldContainSubstring, "Unsupported constraint")
		})
	})
}

func TestConstraintMatches(t *testing.T) {
	Convey("Constraints should match", t, func() {
		So(MustParseConstraint([]string{"LIKE", "^abc$"}).Matches("abc", nil), ShouldBeTrue)
		So(MustParseConstraint([]string{"LIKE", "^abc$"}).Matches("abc1", nil), ShouldBeFalse)

		So(MustParseConstraint([]string{"LIKE", "a.*"}).Matches("abc", nil), ShouldBeTrue)
		So(MustParseConstraint([]string{"LIKE", "a.*"}).Matches("bc", nil), ShouldBeFalse)

		So(MustParseConstraint([]string{"UNIQUE"}).Matches("a", nil), ShouldBeTrue)
		So(MustParseConstraint([]string{"UNIQUE"}).Matches("a", []string{"a"}), ShouldBeFalse)

		So(MustParseConstraint([]string{"CLUSTER"}).Matches("a", nil), ShouldBeTrue)
		So(MustParseConstraint([]string{"CLUSTER"}).Matches("a", []string{"b"}), ShouldBeFalse)

		So(MustParseConstraint([]string{"GROUP_BY"}).Matches("a", []string{"a"}), ShouldBeTrue)
		So(MustParseConstraint([]string{"GROUP_BY"}).Matches("a", []string{"b"}), ShouldBeFalse)
	})
}

func TestConstraintString(t *testing.T) {
	Convey("Constraints should be readable strings", t, func() {
		So(fmt.Sprintf("%s", MustParseConstraint([]string{"LIKE", "abc"})), ShouldEqual, "like:abc")
		So(fmt.Sprintf("%s", MustParseConstraint([]string{"UNLIKE", "abc"})), ShouldEqual, "unlike:abc")
		So(fmt.Sprintf("%s", MustParseConstraint([]string{"UNIQUE"})), ShouldEqual, "unique")
		So(fmt.Sprintf("%s", MustParseConstraint([]string{"CLUSTER"})), ShouldEqual, "cluster")
		So(fmt.Sprintf("%s", MustParseConstraint([]string{"CLUSTER", "123"})), ShouldEqual, "cluster:123")
		So(fmt.Sprintf("%s", MustParseConstraint([]string{"GROUP_BY"})), ShouldEqual, "groupBy")
		So(fmt.Sprintf("%s", MustParseConstraint([]string{"GROUP_BY", "2"})), ShouldEqual, "groupBy:2")
	})
}

func TestMatches(t *testing.T) {
	Convey("Constraints should match properly", t, func() {
		Convey("LIKE", func() {
			like := MustParseConstraint([]string{"LIKE", "^1.*2$"})
			So(like.Matches("12", nil), ShouldBeTrue)
			So(like.Matches("1a2", nil), ShouldBeTrue)
			So(like.Matches("1ab2", nil), ShouldBeTrue)

			So(like.Matches("a1a2", nil), ShouldBeFalse)
			So(like.Matches("1a2a", nil), ShouldBeFalse)
		})

		Convey("UNLIKE", func() {
			unlike := MustParseConstraint([]string{"UNLIKE", "1"})
			So(unlike.Matches("1", nil), ShouldBeFalse)
			So(unlike.Matches("2", nil), ShouldBeTrue)
		})

		Convey("UNIQUE", func() {
			unique := MustParseConstraint([]string{"UNIQUE"})
			So(unique.Matches("1", nil), ShouldBeTrue)
			So(unique.Matches("2", []string{"1"}), ShouldBeTrue)
			So(unique.Matches("3", []string{"1", "2"}), ShouldBeTrue)

			So(unique.Matches("1", []string{"1", "2"}), ShouldBeFalse)
			So(unique.Matches("2", []string{"1", "2"}), ShouldBeFalse)
		})

		Convey("CLUSTER", func() {
			cluster := MustParseConstraint([]string{"CLUSTER"})
			So(cluster.Matches("1", nil), ShouldBeTrue)
			So(cluster.Matches("2", nil), ShouldBeTrue)

			So(cluster.Matches("1", []string{"1"}), ShouldBeTrue)
			So(cluster.Matches("1", []string{"1", "1"}), ShouldBeTrue)
			So(cluster.Matches("2", []string{"1"}), ShouldBeFalse)

			cluster3 := MustParseConstraint([]string{"CLUSTER", "3"})
			So(cluster3.Matches("3", nil), ShouldBeTrue)
			So(cluster3.Matches("2", nil), ShouldBeFalse)

			So(cluster3.Matches("3", []string{"3"}), ShouldBeTrue)
			So(cluster3.Matches("3", []string{"3", "3"}), ShouldBeTrue)
			So(cluster3.Matches("2", []string{"3"}), ShouldBeFalse)
		})

		Convey("GROUP_BY", func() {
			groupBy := MustParseConstraint([]string{"GROUP_BY"})
			So(groupBy.Matches("1", nil), ShouldBeTrue)
			So(groupBy.Matches("1", []string{"1"}), ShouldBeTrue)
			So(groupBy.Matches("1", []string{"1", "1"}), ShouldBeTrue)
			So(groupBy.Matches("1", []string{"2"}), ShouldBeFalse)

			groupBy2 := MustParseConstraint([]string{"GROUP_BY", "2"})
			So(groupBy2.Matches("1", nil), ShouldBeTrue)
			So(groupBy2.Matches("1", []string{"1"}), ShouldBeFalse)
			So(groupBy2.Matches("1", []string{"1", "1"}), ShouldBeFalse)
			So(groupBy2.Matches("2", []string{"1"}), ShouldBeTrue)

			So(groupBy2.Matches("1", []string{"1", "2"}), ShouldBeTrue)
			So(groupBy2.Matches("2", []string{"1", "2"}), ShouldBeTrue)

			So(groupBy2.Matches("1", []string{"1", "1", "2"}), ShouldBeFalse)
			So(groupBy2.Matches("2", []string{"1", "1", "2"}), ShouldBeTrue)
		})
	})
}
