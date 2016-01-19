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

type Context struct {
	lock sync.Mutex
	ctx  map[string]string
}

func NewContext() *Context {
	return &Context{
		ctx: make(map[string]string),
	}
}

func (c *Context) Set(key string, value string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.ctx[key] = value
}

func (c *Context) Get(key string) (string, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	value, ok := c.ctx[key]
	if !ok {
		return "", fmt.Errorf("Key %s is not present", key)
	}

	return value, nil
}

func (c *Context) MustGet(key string) string {
	value, err := c.Get(key)
	if err != nil {
		panic(err)
	}

	return value
}

func (c *Context) All() map[string]string {
	return c.ctx
}

func (c *Context) String() string {
	str, err := json.MarshalIndent(c.ctx, "", "  ")
	if err != nil {
		panic(err)
	}

	return string(str)
}
