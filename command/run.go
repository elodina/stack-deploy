package command

import (
	"fmt"

	"flag"
	"time"

	"github.com/elodina/stack-deploy/api"
)

const defaultApplicationMaxWait = 600 // 10 minutes

type RunCommand struct{}

func (rc *RunCommand) Run(args []string) int {
	if len(args) == 0 {
		fmt.Println("Stack name required to run")
		return 1
	}

	var (
		flags   = flag.NewFlagSet("run", flag.ExitOnError)
		apiUrl  = flags.String("api", "", "Stack-deploy server address.")
		zone    = flags.String("zone", "", "Zone to run stack.")
		maxWait = flags.Int("max.wait", defaultApplicationMaxWait, "Maximum time to wait for each application in a stack to become running and healthy.")
	)
	flags.Parse(args[1:])

	name := args[0]
	stackDeployApi, err := resolveApi(*apiUrl)
	if err != nil {
		fmt.Printf("ERROR resolving API: %s\n", err)
		return 1
	}
	client := api.NewClient(stackDeployApi)

	fmt.Printf("Running stack %s\n", name)
	start := time.Now()
	err = client.Run(name, *zone, *maxWait)
	if err != nil {
		fmt.Printf("ERROR running client request: %s\n", err)
		return 1
	}

	elapsed := time.Since(start)
	fmt.Printf("Done in %s\n", elapsed-elapsed%time.Second)
	return 0
}

func (rc *RunCommand) Help() string {
	return ""
}

func (rc *RunCommand) Synopsis() string {
	return "Run Stack"
}
