package command

import (
	"flag"

	"fmt"

	api "github.com/elodina/stack-deploy/framework"
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

	role := "regular"
	if admin {
		role = "admin"
	}
	key, err := client.CreateUser(&api.CreateUserRequest{
		Name: *name,
		Role: role,
	})
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
