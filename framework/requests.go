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

const DefaultApplicationMaxWait = 600 // 10 minutes

type GetStackRequest struct {
	Name string `json:"name"`
}

type CreateStackRequest struct {
	Stackfile string `json:"stackfile"`
}

type RemoveStackRequest struct {
	Name  string `json:"name"`
	Force bool   `json:"force"`
}

type RunRequest struct {
	Name             string            `json:"name"`
	Zone             string            `json:"zone"`
	MaxWait          int               `json:"maxwait"`
	Variables        map[string]string `json:"variables"`
	SkipApplications []string          `json:"skip"`
}

func NewRunRequest() *RunRequest {
	return &RunRequest{
		MaxWait: DefaultApplicationMaxWait,
	}
}

type CreateLayerRequest struct {
	Stackfile string `json:"stackfile"`
	Layer     string `json:"layer"`
	Parent    string `json:"parent"`
}

type CreateUserRequest struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

type RefreshTokenRequest struct {
	Name string `json:"name"`
}

type RemoveScheduledRequest struct {
	ID int64 `json:"id"`
}
