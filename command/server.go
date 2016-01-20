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
	"flag"
	"os"
	"os/signal"
	"strings"
	"time"

	"fmt"
	"io/ioutil"
	"regexp"

	"github.com/elodina/pyrgus/log"
	"github.com/elodina/stack-deploy/framework"
	marathon "github.com/gambol99/go-marathon"
)

var Logger = log.NewDefaultLogger()

type ServerCommand struct {
	runners map[string]framework.TaskRunner
}

func NewServerCommand(runners map[string]framework.TaskRunner) *ServerCommand {
	return &ServerCommand{
		runners: runners,
	}
}

func (sc *ServerCommand) Run(args []string) int {
	var (
		flags          = flag.NewFlagSet("server", flag.ExitOnError)
		masterURL      = flags.String("master", "127.0.0.1:5050", "Mesos Master address <ip:port>.")
		marathonURL    = flags.String("marathon", "http://127.0.0.1:8080", "Marathon address <ip:port>.")
		api            = flags.String("api", "0.0.0.0:4200", "Stack-deploy server binding address")
		bootstrap      = flags.String("bootstrap", "", "Stack file to bootstrap with.")
		cassandra      = flags.String("cassandra", "127.0.0.1", "Cassandra cluster IPs, comma-separated")
		keyspace       = flags.String("keyspace", "stack_deploy", "Cassandra keyspace")
		connectRetries = flags.Int("connect.retries", 10, "Number of retries to connect to either Marathon or Cassandra")
		connectBackoff = flags.Duration("connect.backoff", 10*time.Second, "Backoff between connection attempts to either Marathon or Cassandra")
		debug          = flags.Bool("debug", false, "Flag for debug mode")
	)
	flags.Parse(args)
	if *debug {
		Logger = log.NewConsoleLogger(log.DebugLevel, log.DefaultLogFormat)
		framework.Logger = log.NewConsoleLogger(log.DebugLevel, log.DefaultLogFormat)
	}

	ctrlc := make(chan os.Signal, 1)
	signal.Notify(ctrlc, os.Interrupt)

	framework.TaskRunners = sc.runners

	framework.Mesos = framework.NewMesosState(*masterURL)
	err := framework.Mesos.Update()
	if err != nil {
		Logger.Critical("%s", err)
		return 1
	}

	marathonClient, err := sc.connectMarathon(*marathonURL, *connectRetries, *connectBackoff)
	if err != nil {
		Logger.Critical("%s", err)
		return 1
	}

	if *bootstrap != "" {
		context, err := sc.Bootstrap(*bootstrap, marathonClient, *connectRetries, *connectBackoff)
		if err != nil {
			Logger.Critical("%s", err)
			return 1
		}
		Logger.Info("Bootstrap context: %s", context)
		Logger.Info("Cassandra connect before resolving: %s", *cassandra)

		for k, v := range context.All() {
			*cassandra = strings.Replace(*cassandra, fmt.Sprintf("{%s}", fmt.Sprint(k)), fmt.Sprint(v), -1)
		}
		Logger.Info("Cassandra connect after resolving: %s", *cassandra)
	}

	storage, connection, err := framework.NewCassandraStorageRetryBackoff(strings.Split(*cassandra, ","), *keyspace, *connectRetries, *connectBackoff)
	if err != nil {
		panic(err)
	}

	userStorage, key, err := framework.NewCassandraUserStorage(connection, *keyspace)
	if err != nil {
		panic(err)
	}
	stateStorage, err := framework.NewCassandraStateStorage(connection, *keyspace)
	if err != nil {
		panic(err)
	}

	apiServer := framework.NewApiServer(*api, marathonClient, storage, userStorage, stateStorage)
	if key != "" {
		fmt.Printf("***\nAdmin user key: %s\n***\n", key)
	}
	go apiServer.Start()

	<-ctrlc
	return 0
}

func (sc *ServerCommand) Bootstrap(stackFile string, marathonClient marathon.Marathon, retries int, backoff time.Duration) (*framework.Context, error) {
	stackFileData, err := ioutil.ReadFile(stackFile)
	if err != nil {
		Logger.Error("Can't read file %s", stackFile)
		return nil, err
	}

	stack, err := framework.UnmarshalStack(stackFileData)
	if err != nil {
		return nil, err
	}

	Logger.Debug("Boostrapping with stack: \n%s", string(stackFileData))

	var context *framework.Context
	bootstrapZone := ""
	for i := 0; i < retries; i++ {
		context, err = stack.Run(bootstrapZone, marathonClient, nil)
		if err == nil {
			return context, err
		}

		if err != nil && !regexp.MustCompile(marathon.ErrMarathonDown.Error()).MatchString(err.Error()) {
			return nil, err
		}
		time.Sleep(backoff)
	}

	return context, err
}

func (sc *ServerCommand) connectMarathon(url string, retries int, backoff time.Duration) (marathon.Marathon, error) {
	var err error
	var marathonClient marathon.Marathon
	for i := 0; i < retries; i++ {
		Logger.Info("Trying to connect to Marathon: attempt %d", i)
		marathonClient, err = sc.newMarathonClient(url)
		if err == nil {
			return marathonClient, nil
		}
		Logger.Debug("Error: %s", err)
		time.Sleep(backoff)
	}
	return nil, err
}

func (sc *ServerCommand) newMarathonClient(url string) (marathon.Marathon, error) {
	marathonConfig := marathon.NewDefaultConfig()
	marathonConfig.URL = url
	marathonClient, err := marathon.NewClient(marathonConfig)
	if err != nil {
		return nil, err
	}

	return marathonClient, nil
}

func (sc *ServerCommand) Help() string     { return "Starts stack-deploy Server" }
func (sc *ServerCommand) Synopsis() string { return "Starts stack-deploy Server" }
