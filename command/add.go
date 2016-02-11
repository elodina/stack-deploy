package command

import (
	"flag"
	"io/ioutil"

	"fmt"

	api "github.com/elodina/stack-deploy/framework"
)

type AddStackCommand struct{}

func (asc *AddStackCommand) Run(args []string) int {
	var (
		flags     = flag.NewFlagSet("add", flag.ExitOnError)
		apiUrl    = flags.String("api", "", "Stack-deploy server address.")
		stackFile = flags.String("file", "Stackfile", "Stackfile with Application Configs")
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
	err = client.CreateStack(&api.CreateStackRequest{
		Stackfile: string(stack),
	})
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		return 1
	}

	fmt.Println("Stack added")
	return 0
}

func (asc *AddStackCommand) Help() string {
	return ""
}

func (asc *AddStackCommand) Synopsis() string {
	return "Add new stack"
}
