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

type StackContext struct {
	lock               sync.RWMutex
	stackVariables     map[string]string
	arbitraryVariables map[string]string
	globalVariables    map[string]string
}

func NewContext() *StackContext {
	return &StackContext{
		stackVariables:     make(map[string]string),
		arbitraryVariables: make(map[string]string),
		globalVariables:    make(map[string]string),
	}
}

func (c *StackContext) SetStackVariable(key string, value string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.stackVariables[key] = value
}

func (c *StackContext) SetArbitraryVariable(key string, value string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.arbitraryVariables[key] = value
}

func (c *StackContext) SetGlobalVariable(key string, value string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.globalVariables[key] = value
}

func (c *StackContext) Get(key string) (string, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	value, ok := c.stackVariables[key]
	if ok {
		return value, nil
	}

	value, ok = c.arbitraryVariables[key]
	if ok {
		return value, nil
	}

	value, ok = c.globalVariables[key]
	if ok {
		return value, nil
	}

	return "", fmt.Errorf("Key %s is not present", key)
}

func (c *StackContext) MustGet(key string) string {
	value, err := c.Get(key)
	if err != nil {
		panic(err)
	}

	return value
}

func (c *StackContext) All() map[string]string {
	allVariables := make(map[string]string)
	for k, v := range c.globalVariables {
		allVariables[k] = v
	}

	for k, v := range c.arbitraryVariables {
		allVariables[k] = v
	}

	for k, v := range c.stackVariables {
		allVariables[k] = v
	}

	return allVariables
}

func (c *StackContext) String() string {
	str, err := json.MarshalIndent(map[string]interface{}{
		"StackVariables":     c.stackVariables,
		"ArbitraryVariables": c.arbitraryVariables,
		"GlobalVariables":    c.globalVariables,
	}, "", "  ")
	if err != nil {
		panic(err)
	}

	return string(str)
}

func (c *StackContext) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]map[string]string{
		"StackVariables":     c.stackVariables,
		"ArbitraryVariables": c.arbitraryVariables,
		"GlobalVariables":    c.globalVariables,
	})
}

func (c *StackContext) UnmarshalJSON(data []byte) error {
	ctx := make(map[string]map[string]string)
	err := json.Unmarshal(data, &ctx)
	if err != nil {
		return err
	}

	for k, v := range ctx["StackVariables"] {
		c.SetStackVariable(k, v)
	}

	for k, v := range ctx["ArbitraryVariables"] {
		c.SetArbitraryVariable(k, v)
	}

	for k, v := range ctx["GlobalVariables"] {
		c.SetGlobalVariable(k, v)
	}

	return nil
}
