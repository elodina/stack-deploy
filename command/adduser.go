package command

import (
	"flag"

	"fmt"

	"github.com/elodina/stack-deploy/api"
)

type AddUserCommand struct{}

func (auc *AddUserCommand) Run(args []string) int {
	var (
		flags  = flag.NewFlagSet("adduser", flag.ExitOnError)
		apiUrl = flags.String("api", "", "Stack-deploy server address.")
		name   = flags.String("name", "", "New user name")
		admin  = flags.Bool("admin", false, "Create admin")
	)
	flags.Parse(args)

	stackDeployApi, err := resolveApi(*apiUrl)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		return 1
	}

	client := api.NewClient(stackDeployApi)
	key, err := client.CreateUser(*name, *admin)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		return 1
	}

	fmt.Printf("User added. Key: %s\n", key)
	return 0
}

func (auc *AddUserCommand) Help() string {
	return ""
}

func (auc *AddUserCommand) Synopsis() string {
	return "Add new user"
}
