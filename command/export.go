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
	"io/ioutil"
)

type ExportCommand struct{}

func (ec *ExportCommand) Run(args []string) int {
	var (
		flags  = flag.NewFlagSet("export", flag.ExitOnError)
		apiUrl = flags.String("api", "", "Stack-deploy server address.")
		file   = flags.String("file", "state.json", "File name to output state to.")
	)
	flags.Parse(args)

	stackDeployApi, err := resolveApi(*apiUrl)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		return 1
	}
	client := api.NewClient(stackDeployApi)

	state, err := client.GetState()
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		return 1
	}

	err = ioutil.WriteFile(*file, state, 0777)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		return 1
	}

	return 0
}

func (ec *ExportCommand) Help() string {
	return ""
}

func (ec *ExportCommand) Synopsis() string {
	return "Export stack-deploy state to file"
}
