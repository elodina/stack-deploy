package command

import (
	"flag"

	"fmt"

	"github.com/elodina/stack-deploy/api"
)

type RefreshTokenCommand struct{}

func (*RefreshTokenCommand) Run(args []string) int {
	var (
		flags  = flag.NewFlagSet("refreshtoken", flag.ExitOnError)
		apiUrl = flags.String("api", "", "Stack-deploy server address.")
		name   = flags.String("name", "", "New user name")
	)
	flags.Parse(args)

	stackDeployApi, err := resolveApi(*apiUrl)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		return 1
	}

	client := api.NewClient(stackDeployApi)
	key, err := client.RefreshToken(*name)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		return 1
	}

	fmt.Printf("New key generated: %s\n", key)
	return 0
}

func (*RefreshTokenCommand) Help() string {
	return ""
}

func (*RefreshTokenCommand) Synopsis() string {
	return "Refresh user token"
}
