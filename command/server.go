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
	"github.com/gocql/gocql"
)

var Logger = log.NewDefaultLogger()

type ServerCommand struct {
	runners      map[string]framework.TaskRunner
	mesosRunners map[string]framework.MesosTaskRunner
}

func NewServerCommand(runners map[string]framework.TaskRunner, mesosRunners map[string]framework.MesosTaskRunner) *ServerCommand {
	return &ServerCommand{
		runners:      runners,
		mesosRunners: mesosRunners,
	}
}

func (sc *ServerCommand) Run(args []string) int {
	schedulerConfig := framework.NewSchedulerConfig()

	var (
		flags             = flag.NewFlagSet("server", flag.ExitOnError)
		marathonURL       = flags.String("marathon", "http://127.0.0.1:8080", "Marathon address <ip:port>.")
		persistentStorage = flags.String("storage", "", "Storage to store stack-deploy runtime information to recover from failovers. Required.")
		api               = flags.String("api", "0.0.0.0:4200", "Stack-deploy server binding address")
		bootstrap         = flags.String("bootstrap", "", "Stack file to bootstrap with.")
		cassandra         = flags.String("cassandra", "127.0.0.1", "Cassandra cluster IPs, comma-separated")
		keyspace          = flags.String("keyspace", "stack_deploy", "Cassandra keyspace")
		proto             = flags.Int("proto", 3, "Cassandra protocol version")
		connectRetries    = flags.Int("connect.retries", 10, "Number of retries to connect to either Marathon or Cassandra")
		connectBackoff    = flags.Duration("connect.backoff", 10*time.Second, "Backoff between connection attempts to either Marathon or Cassandra")
		debug             = flags.Bool("debug", false, "Flag for debug mode")
		dev               = flags.Bool("dev", false, "Flag for developer mode")
		variables         = make(vars)
	)
	flags.StringVar(&schedulerConfig.Master, "master", "127.0.0.1:5050", "Mesos Master address <ip:port>.")
	flags.StringVar(&schedulerConfig.User, "framework.user", "", "Mesos user. Defaults to current system user.")
	flags.StringVar(&schedulerConfig.FrameworkName, "framework.name", "stack-deploy", "Mesos framework name. Defaults to stack-deploy.")
	flags.StringVar(&schedulerConfig.FrameworkRole, "framework.role", "*", "Mesos framework role. Defaults to *.")
	flags.DurationVar(&schedulerConfig.FailoverTimeout, "failover.timeout", 168*time.Hour, "Mesos framework failover timeout. Defaults to 1 week.")
	flags.Var(variables, "var", "Global variables to add to every stack context run by stack-deploy server. Multiple occurrences of this flag allowed.")

	flags.Parse(args)
	if *debug {
		Logger = log.NewConsoleLogger(log.DebugLevel, log.DefaultLogFormat)
		framework.Logger = log.NewConsoleLogger(log.DebugLevel, log.DefaultLogFormat)
	}

	ctrlc := make(chan os.Signal, 1)
	signal.Notify(ctrlc, os.Interrupt)

	framework.TaskRunners = sc.runners
	framework.MesosTaskRunners = sc.mesosRunners

	marathonClient, err := sc.connectMarathon(*marathonURL, *connectRetries, *connectBackoff)
	if err != nil {
		Logger.Critical("%s", err)
		return 1
	}

	if *persistentStorage == "" {
		if !*dev {
			Logger.Critical("--storage flag is required. Examples: 'file:stack-deploy.json', 'zk:zookeeper.service:2181/stack-deploy'")
			return 1
		} else {
			*persistentStorage = "file:stack-deploy.json"
		}
	}

	frameworkStorage, err := framework.NewFrameworkStorage(*persistentStorage)
	if err != nil {
		Logger.Critical("%s", err)
		return 1
	}
	schedulerConfig.Storage = frameworkStorage
	frameworkStorage.Load()

	scheduler := framework.NewScheduler(schedulerConfig)
	err = scheduler.GetMesosState().Update()
	if err != nil {
		Logger.Critical("%s", err)
		return 1
	}

	if *bootstrap != "" {
		var context *framework.StackContext
		if frameworkStorage.BootstrapContext != nil && len(frameworkStorage.BootstrapContext.All()) > 0 {
			Logger.Info("Restored bootstrap context from persistent storage")
			context = frameworkStorage.BootstrapContext
		} else {
			Logger.Info("No existing bootstrap context found in persistent storage, bootstrapping")
			var err error
			context, err = sc.Bootstrap(*bootstrap, marathonClient, scheduler, *connectRetries, *connectBackoff)
			if err != nil {
				Logger.Critical("%s", err)
				return 1
			}
		}

		Logger.Info("Bootstrap context: %s", context)
		Logger.Info("Cassandra connect before resolving: %s", *cassandra)
		for k, v := range context.All() {
			*cassandra = strings.Replace(*cassandra, fmt.Sprintf("{%s}", fmt.Sprint(k)), fmt.Sprint(v), -1)
		}
		Logger.Info("Cassandra connect after resolving: %s", *cassandra)
		frameworkStorage.BootstrapContext = context
		frameworkStorage.Save()
	}

	var storage framework.Storage
	var userStorage framework.UserStorage
	var stateStorage framework.StateStorage
	var key string
	if !*dev {
		var connection *gocql.Session
		var err error
		storage, connection, err = framework.NewCassandraStorageRetryBackoff(strings.Split(*cassandra, ","), *keyspace, *connectRetries, *connectBackoff, *proto)
		if err != nil {
			panic(err)
		}

		userStorage, key, err = framework.NewCassandraUserStorage(connection, *keyspace)
		if err != nil {
			panic(err)
		}

		stateStorage, err = framework.NewCassandraStateStorage(connection, *keyspace)
		if err != nil {
			panic(err)
		}
	} else {
		Logger.Warn("Starting in developer mode. DO NOT use this in production!")
		schedulerConfig.FailoverTimeout = time.Duration(0)
		storage = framework.NewInMemoryStorage()
		userStorage = new(framework.NoopUserStorage)
		stateStorage = new(framework.NoopStateStorage)
	}

	apiServer := framework.NewApiServer(*api, marathonClient, variables, storage, userStorage, stateStorage, scheduler, *dev)
	if key != "" {
		fmt.Printf("***\nAdmin user key: %s\n***\n", key)
	}
	go apiServer.Start()

	<-ctrlc
	return 0
}

func (sc *ServerCommand) Bootstrap(stackFile string, marathonClient marathon.Marathon, scheduler framework.Scheduler, retries int, backoff time.Duration) (*framework.StackContext, error) {
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

	var context *framework.StackContext
	bootstrapZone := ""
	for i := 0; i < retries; i++ {
		context, err = stack.Run(&framework.RunRequest{
			Zone:    bootstrapZone,
			MaxWait: defaultApplicationMaxWait,
		}, framework.NewContext(), marathonClient, scheduler, new(framework.NoopStateStorage))
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
