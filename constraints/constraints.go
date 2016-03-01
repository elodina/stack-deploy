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
	"errors"
	"fmt"
	mesos "github.com/mesos/mesos-go/mesosproto"
	"math"
	"regexp"
	"strconv"
)

type Constraint interface {
	Matches(value string, values []string) bool
}

func ParseConstraint(value []string) (Constraint, error) {
	if len(value) == 0 {
		return nil, fmt.Errorf("Unsupported constraint: %v", value)
	} else {
		switch value[0] {
		case "LIKE":
			if len(value) < 2 {
				return nil, fmt.Errorf("Unsupported constraint: %v", value)
			}
			return NewLikeConstraint(value[1])
		case "UNLIKE":
			if len(value) < 2 {
				return nil, fmt.Errorf("Unsupported constraint: %v", value)
			}
			return NewUnlikeConstraint(value[1])
		case "UNIQUE":
			return NewUniqueConstraint(), nil
		case "CLUSTER":
			if len(value) < 2 {
				return NewClusterConstraint(""), nil
			}
			return NewClusterConstraint(value[1]), nil
		case "GROUP_BY":
			if len(value) < 2 {
				return NewGroupByConstraint(1), nil
			}
			groups, err := strconv.Atoi(value[1])
			if err != nil {
				return nil, fmt.Errorf("Invalid constraint: %v", value)
			}
			return NewGroupByConstraint(groups), nil
		default:
			return nil, fmt.Errorf("Unsupported constraint: %v", value)
		}
	}
}

func MustParseConstraint(value []string) Constraint {
	constraint, err := ParseConstraint(value)
	if err != nil {
		panic(err)
	}

	return constraint
}

func ParseConstraints(rawConstraints [][]string) (map[string][]Constraint, error) {
	if len(rawConstraints) == 0 {
		return nil, nil
	}

	constraints := make(map[string][]Constraint)
	for _, rawConstraint := range rawConstraints {
		if len(rawConstraint) == 0 {
			return nil, errors.New("Invalid constraint")
		}
		constraint, err := ParseConstraint(rawConstraint[1:])
		if err != nil {
			return nil, err
		}

		attribute := rawConstraint[0]
		_, exists := constraints[attribute]
		if !exists {
			constraints[attribute] = make([]Constraint, 0)
		}
		constraints[attribute] = append(constraints[attribute], constraint)
	}

	return constraints, nil
}

type Like struct {
	regex   string
	pattern *regexp.Regexp
}

func NewLikeConstraint(regex string) (*Like, error) {
	pattern, err := regexp.Compile(regex)
	if err != nil {
		return nil, fmt.Errorf("Invalid like: %s", err)
	}

	return &Like{
		regex:   regex,
		pattern: pattern,
	}, nil
}

func (l *Like) Matches(value string, values []string) bool {
	return l.pattern.MatchString(value)
}

func (l *Like) String() string {
	return fmt.Sprintf("like:%s", l.regex)
}

type Unlike struct {
	regex   string
	pattern *regexp.Regexp
}

func NewUnlikeConstraint(regex string) (*Unlike, error) {
	pattern, err := regexp.Compile(regex)
	if err != nil {
		return nil, fmt.Errorf("Invalid unlike: %s", err)
	}

	return &Unlike{
		regex:   regex,
		pattern: pattern,
	}, nil
}

func (u *Unlike) Matches(value string, values []string) bool {
	return !u.pattern.MatchString(value)
}

func (u *Unlike) String() string {
	return fmt.Sprintf("unlike:%s", u.regex)
}

type Unique struct{}

func NewUniqueConstraint() *Unique {
	return new(Unique)
}

func (u *Unique) Matches(value string, values []string) bool {
	for _, v := range values {
		if value == v {
			return false
		}
	}

	return true
}

func (u *Unique) String() string {
	return "unique"
}

type Cluster struct {
	value string
}

func NewClusterConstraint(value string) *Cluster {
	return &Cluster{
		value: value,
	}
}

func (c *Cluster) Matches(value string, values []string) bool {
	if c.value != "" {
		return c.value == value
	} else {
		return len(values) == 0 || values[0] == value
	}
}

func (c *Cluster) String() string {
	if c.value != "" {
		return fmt.Sprintf("cluster:%s", c.value)
	} else {
		return "cluster"
	}
}

type GroupBy struct {
	groups int
}

func NewGroupByConstraint(groups int) *GroupBy {
	return &GroupBy{
		groups: groups,
	}
}

func (g *GroupBy) Matches(value string, values []string) bool {
	counts := make(map[string]int)
	for _, v := range values {
		counts[v] = counts[v] + 1
	}

	if len(counts) < g.groups {
		_, ok := counts[value]
		return !ok
	} else {
		minCount := int(math.MaxInt32)
		for _, v := range counts {
			if v < minCount {
				minCount = v
			}
		}
		if minCount == int(math.MaxInt32) {
			minCount = 0
		}

		return counts[value] == minCount
	}
}

func (g *GroupBy) String() string {
	if g.groups > 1 {
		return fmt.Sprintf("groupBy:%d", g.groups)
	} else {
		return "groupBy"
	}
}

func OfferAttributes(offer *mesos.Offer) map[string]string {
	offerAttributes := map[string]string{
		"hostname": offer.GetHostname(),
	}

	for _, attribute := range offer.GetAttributes() {
		text := attribute.GetText().GetValue()
		if text != "" {
			offerAttributes[attribute.GetName()] = text
		}
	}

	return offerAttributes
}
