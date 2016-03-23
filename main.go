package main

import (
	"fmt"
	"os"

	"github.com/elodina/stack-deploy/command"
	"github.com/elodina/stack-deploy/framework"
	"github.com/elodina/stack-deploy/mesosrunners"
	"github.com/elodina/stack-deploy/runners"
	"github.com/mitchellh/cli"
)

func main() {
	cli := cli.NewCLI("stack-deploy", "0.3.4.1")
	cli.Args = os.Args[1:]
	cli.Commands = commands()

	exitCode, err := cli.Run()
	if err != nil {
		fmt.Printf("Error exiting CLI: %s\n", err)
		os.Exit(1)
	}

	os.Exit(exitCode)
}

func commands() map[string]cli.CommandFactory {
	return map[string]cli.CommandFactory{
		"server": func() (cli.Command, error) {
			return command.NewServerCommand(taskRunners, mesosTaskRunners), nil
		},
		"ping": func() (cli.Command, error) {
			return new(command.PingCommand), nil
		},
		"list": func() (cli.Command, error) {
			return new(command.ListCommand), nil
		},
		"show": func() (cli.Command, error) {
			return new(command.ShowCommand), nil
		},
		"run": func() (cli.Command, error) {
			return new(command.RunCommand), nil
		},
		"add": func() (cli.Command, error) {
			return new(command.AddStackCommand), nil
		},
		"remove": func() (cli.Command, error) {
			return new(command.RemoveStackCommand), nil
		},
		"adduser": func() (cli.Command, error) {
			return new(command.AddUserCommand), nil
		},
		"refreshtoken": func() (cli.Command, error) {
			return new(command.RefreshTokenCommand), nil
		},
		"addlayer": func() (cli.Command, error) {
			return new(command.AddLayerCommand), nil
		},
	}
}

// Register your custom task runners here
var taskRunners map[string]framework.TaskRunner = map[string]framework.TaskRunner{
	"kafka-mesos-0.9.x":           new(runners.KafkaTaskRunner),
	"exhibitor-mesos-0.1.x":       new(runners.ExhibitorTaskRunner),
	"dse-mesos-0.1.x":             new(runners.DSETaskRunner),
	"dse-mesos-0.2.x":             new(runners.DSE02xTaskRunner),
	"statsd-mesos-0.1.x":          new(runners.StatsdTaskRunner),
	"syslog-mesos-0.1.x":          new(runners.SyslogTaskRunner),
	"zipkin-mesos-0.1.x":          new(runners.ZipkinTaskRunner),
	"go-kafka-client-mesos-0.3.x": new(runners.GoKafkaClientTaskRunner),
}

var mesosTaskRunners map[string]framework.MesosTaskRunner = map[string]framework.MesosTaskRunner{
	"run-once": mesosrunners.NewRunOnceRunner(),
}
