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

package command

import (
	"fmt"

	"flag"
	"strings"

	api "github.com/elodina/stack-deploy/framework"
)

type ListCommand struct{}

func (lc *ListCommand) Run(args []string) int {
	var (
		flags  = flag.NewFlagSet("run", flag.ExitOnError)
		apiUrl = flags.String("api", "", "Stack-deploy server address.")
	)
	flags.Parse(args)

	stackDeployApi, err := resolveApi(*apiUrl)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		return 1
	}

	client := api.NewClient(stackDeployApi)

	stacks, err := client.List()
	if err != nil {
		fmt.Printf("ERROR getting list: %s\n", err)
		return 1
	}

	fmt.Println(strings.Join(stacks, "\n"))
	return 0
}

func (lc *ListCommand) Help() string {
	return ""
}

func (lc *ListCommand) Synopsis() string {
	return "List stacks"
}
