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

	api "github.com/elodina/stack-deploy/framework"
)

type ShowCommand struct{}

func (sc *ShowCommand) Run(args []string) int {
	if len(args) == 0 {
		fmt.Println("Stack name required to show")
		return 1
	}

	var (
		flags  = flag.NewFlagSet("run", flag.ExitOnError)
		apiUrl = flags.String("api", "", "Stack-deploy server address.")
	)
	flags.Parse(args[1:])

	name := args[0]

	stackDeployApi, err := resolveApi(*apiUrl)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		return 1
	}
	client := api.NewClient(stackDeployApi)

	stack, err := client.GetStack(&api.GetStackRequest{
		Name: name,
	})
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		return 1
	}

	fmt.Println(stack)
	return 0
}

func (sc *ShowCommand) Help() string {
	return ""
}

func (sc *ShowCommand) Synopsis() string {
	return "Show stack"
}
