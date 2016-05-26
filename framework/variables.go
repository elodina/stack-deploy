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
	"encoding/json"
	"fmt"
	"sync"
)

type Variables struct {
	lock               sync.RWMutex
	stackVariables     map[string]string
	arbitraryVariables map[string]string
	globalVariables    map[string]string
}

func NewVariables() *Variables {
	return &Variables{
		stackVariables:     make(map[string]string),
		arbitraryVariables: make(map[string]string),
		globalVariables:    make(map[string]string),
	}
}

func (v *Variables) SetStackVariable(key string, value string) {
	v.lock.Lock()
	defer v.lock.Unlock()

	v.stackVariables[key] = value
}

func (v *Variables) SetArbitraryVariable(key string, value string) {
	v.lock.Lock()
	defer v.lock.Unlock()

	v.arbitraryVariables[key] = value
}

func (v *Variables) SetGlobalVariable(key string, value string) {
	v.lock.Lock()
	defer v.lock.Unlock()

	v.globalVariables[key] = value
}

func (v *Variables) Get(key string) (string, error) {
	v.lock.RLock()
	defer v.lock.RUnlock()

	value, ok := v.stackVariables[key]
	if ok {
		return value, nil
	}

	value, ok = v.arbitraryVariables[key]
	if ok {
		return value, nil
	}

	value, ok = v.globalVariables[key]
	if ok {
		return value, nil
	}

	return "", fmt.Errorf("Key %s is not present", key)
}

func (v *Variables) MustGet(key string) string {
	value, err := v.Get(key)
	if err != nil {
		panic(err)
	}

	return value
}

func (v *Variables) All() map[string]string {
	allVariables := make(map[string]string)
	for k, v := range v.globalVariables {
		allVariables[k] = v
	}

	for k, v := range v.arbitraryVariables {
		allVariables[k] = v
	}

	for k, v := range v.stackVariables {
		allVariables[k] = v
	}

	return allVariables
}

func (v *Variables) String() string {
	str, err := json.MarshalIndent(map[string]interface{}{
		"StackVariables":     v.stackVariables,
		"ArbitraryVariables": v.arbitraryVariables,
		"GlobalVariables":    v.globalVariables,
	}, "", "  ")
	if err != nil {
		panic(err)
	}

	return string(str)
}

func (v *Variables) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]map[string]string{
		"StackVariables":     v.stackVariables,
		"ArbitraryVariables": v.arbitraryVariables,
		"GlobalVariables":    v.globalVariables,
	})
}

func (v *Variables) UnmarshalJSON(data []byte) error {
	ctx := make(map[string]map[string]string)
	err := json.Unmarshal(data, &ctx)
	if err != nil {
		return err
	}

	v.stackVariables = make(map[string]string)
	v.arbitraryVariables = make(map[string]string)
	v.globalVariables = make(map[string]string)

	for key, value := range ctx["StackVariables"] {
		v.SetStackVariable(key, value)
	}

	for key, value := range ctx["ArbitraryVariables"] {
		v.SetArbitraryVariable(key, value)
	}

	for key, value := range ctx["GlobalVariables"] {
		v.SetGlobalVariable(key, value)
	}

	return nil
}
