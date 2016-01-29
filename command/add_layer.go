package command

import (
	"flag"
	"io/ioutil"

	"fmt"

	"github.com/elodina/stack-deploy/api"
)

type AddLayerCommand struct{}

func (asc *AddLayerCommand) Run(args []string) int {
	var (
		flags     = flag.NewFlagSet("add", flag.ExitOnError)
		apiUrl    = flags.String("api", "", "Stack-deploy server address.")
		stackFile = flags.String("file", "Stackfile", "Stackfile with Application Configs")
		level     = flags.String("level", "zone", "zone|cluster|datacenter")
		parent    = flags.String("parent", "", "Parent layer")
	)
	flags.Parse(args)

	stackDeployApi, err := resolveApi(*apiUrl)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		return 1
	}

	client := api.NewClient(stackDeployApi)
	stack, err := ioutil.ReadFile(*stackFile)
	if err != nil {
		fmt.Printf("Can't read file %s\n", *stackFile)
		return 1
	}
	err = client.CreateLayer(string(stack), *level, *parent)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		return 1
	}

	fmt.Println("Stack added")
	return 0
}

func (asc *AddLayerCommand) Help() string {
	return ""
}

func (asc *AddLayerCommand) Synopsis() string {
	return "Add new stack"
}
