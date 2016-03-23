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
	"github.com/elodina/go-mesos-utils"
	"strings"
)

type FrameworkStorage struct {
	FrameworkID      string
	BootstrapContext *StackContext

	storage utils.Storage
}

func NewFrameworkStorage(storage string) (*FrameworkStorage, error) {
	Logger.Info("Connecting to persistent storage %s", storage)
	store, err := NewStorage(storage)
	if err != nil {
		return nil, err
	}
	return &FrameworkStorage{
		storage:          store,
		BootstrapContext: NewContext(),
	}, nil
}

func (s *FrameworkStorage) Save() {
	js, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}

	s.storage.Save(js)
}

func (s *FrameworkStorage) Load() {
	js, err := s.storage.Load()
	if err != nil || js == nil {
		Logger.Warn("Could not load cluster state from %s, assuming no cluster state available...", s.storage)
		return
	}

	err = json.Unmarshal(js, &s)
	if err != nil {
		panic(err)
	}
}

func NewStorage(storage string) (utils.Storage, error) {
	storageTokens := strings.SplitN(storage, ":", 2)
	if len(storageTokens) != 2 {
		return nil, fmt.Errorf("Unsupported storage")
	}

	switch storageTokens[0] {
	case "file":
		return utils.NewFileStorage(storageTokens[1]), nil
	case "zk":
		return utils.NewZKStorage(storageTokens[1])
	default:
		return nil, fmt.Errorf("Unsupported storage")
	}
}
