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

import "errors"

var ErrApplicationNoType = errors.New("No application type")

var ErrApplicationNoTaskRunner = errors.New("No task runner available for application")

var ErrApplicationNoID = errors.New("No application ID")

var ErrApplicationInvalidCPU = errors.New("Invalid application CPU")

var ErrApplicationInvalidMem = errors.New("Invalid application Mem")

var ErrApplicationNoLaunchCommand = errors.New("No launch command specified for application")

var ErrApplicationInvalidInstances = errors.New("Invalid number of application instances: supported are numbers greater than zero and 'all'")
