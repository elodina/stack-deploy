package command

import (
	"flag"

	"fmt"

	"github.com/elodina/stack-deploy/api"
)

type PingCommand struct{}

func (*PingCommand) Run(args []string) int {
	var (
		flags  = flag.NewFlagSet("ping", flag.ExitOnError)
		apiUrl = flags.String("api", "", "Stack-deploy server address.")
	)
	flags.Parse(args)

	stackDeployApi, err := resolveApi(*apiUrl)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		return 1
	}

	client := api.NewClient(stackDeployApi)
	err = client.Ping()
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		return 1
	}
	return 0
}

func (*PingCommand) Help() string {
	return ""
}

func (*PingCommand) Synopsis() string {
	return "Ping stack-deploy server"
}
