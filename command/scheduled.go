package command

import (
	"flag"

	"fmt"

	api "github.com/elodina/stack-deploy/framework"
)

type ScheduledCommand struct{}

func (*ScheduledCommand) Run(args []string) int {
	var (
		flags  = flag.NewFlagSet("scheduled", flag.ExitOnError)
		apiUrl = flags.String("api", "", "Stack-deploy server address.")
		remove = flags.Int64("remove", -1, "Remove scheduled task ID")
	)
	flags.Parse(args)

	stackDeployApi, err := resolveApi(*apiUrl)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		return 1
	}

	client := api.NewClient(stackDeployApi)
	if *remove == -1 {
		tasks, err := client.Scheduled()
		if err != nil {
			fmt.Printf("ERROR: %s\n", err)
			return 1
		}
		for _, task := range tasks {
			fmt.Printf("[%d] %s\n\tstart: '%s'\n\tschedule: '%s'\n", task.ID, task.Name, task.StartTime, task.TimeSchedule)
		}
		return 0
	}
	resp, err := client.RemoveScheduled(&api.RemoveScheduledRequest{ID: *remove})
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		return 1
	}
	fmt.Println(resp)
	return 0
}

func (*ScheduledCommand) Help() string {
	return ""
}

func (*ScheduledCommand) Synopsis() string {
	return "View scheduled tasks"
}
