stack-deploy
==========

[Installation](#installation)
* [Prerequisites](#prerequisites)
* [Start the server](#start-the-server)
* [Running with Marathon](#running-with-marathon)    

[Usage](#usage)
* [The concept of stack](#the-concept-of-stack)
* [Stack inheritance](#stack-inheritance)
* [Adding stacks](#adding-stacks)
* [Removing stacks](#removing-stacks)
* [Listing stacks](#listing-stacks)
* [Showing stacks](#showing-stacks)
* [Running stacks](#running-stacks)
* [Run-once tasks](#run-once-tasks)
* [Running Docker containers](#running-docker-containers)
* [Minimal stack examples](#minimal-stack-examples)
* [Task Runners](https://github.com/elodina/stack-deploy/blob/master/docs/task_runners.md)
* [Developer Mode](#developer-mode)

[Changelog](https://github.com/elodina/stack-deploy/blob/master/CHANGELOG.md)

Installation
============
Prerequisites
---------------

Install go 1.4 (or higher) http://golang.org/doc/install

Install godep https://github.com/tools/godep

Clone and build the project

    # git clone https://github.com/elodina/stack-deploy.git
    # cd stack-deploy
    # godep restore ./...
    # go build

Start the server
------------------

stack-deploy requires Mesos, Marathon and Cassandra to work. Cassandra though is not quite required during startup because stack-deploy supports bootstrapping.

In case you have Cassandra running somewhere you may run the stack-deploy server like this:

    # ./stack-deploy server --master master:5050 --marathon http://master:8080 --cassandra cassandra.service

In case you wish stack-deploy to bootstrap you should configure a bootstrap stack which will be run before actually starting the server. A sample bootstrap stack is available in bootstrap.stack file. Please note that bootstrap stacks do not allow inheritance (should not have `from` field). To run stack-deploy bootstrapping with some stack do this:

    # ./stack-deploy server --master master:5050 --marathon http://master:8080 --cassandra {dse-mesos.cassandraConnect} --bootstrap bootstrap.stack

Available flags:

`--master` - [`127.0.0.1:5050`] - Mesos Master address [host]:[port].    
`--marathon` - [`http://127.0.0.1:8080`] - Marathon address [host]:[port].    
`--storage` - Required flag to provide persistent storage to recover from failovers. *Required in prod mode*. Examples: `file:stack-deploy.json`, `zk:zookeeper.service:2181/stack-deploy`    
`--var` - Global variables to add to every stack context run by stack-deploy server. Multiple occurrences of this flag allowed.    
`--api` - [`0.0.0.0:4200`] - stack-deploy server bind address.    
`--bootstrap` - Stack file to bootstrap with.    
`--cassandra` - [`127.0.0.1`] - Cassandra cluster IP addresses, comma separated.    
`--keyspace` - [`stack-deploy`] - Cassandra keyspace.    
`--connect.retries` - [`10`] - Number of retries to connect either to Marathon or Cassandra.    
`--connect.backoff` - [`10s`] - Backoff between connection attempts to either Marathon or Cassandra.    
`--framework.user` - [empty string] - Mesos user. Defaults to current system user.    
`--framework.name` - [`stack-deploy`] - Mesos framework name.    
`--framework.role` - [`*`] - Mesos framework role.    
`--failover.timeout` - [`168 * time.Hour`] - Mesos framework failover timeout. Defaults to 1 week.    
`--debug` - [`false`] - Flag for debug mode.    
`--dev` - [`false`] - Flag for [developer mode](#developer-mode).

To check the server is started, run:
```
# ./stack-deploy ping
Pong
```

First Start and User Management
---------------------------

NOTE: this does not apply for [developer mode](#developer-mode).

If you are running stack-deploy on empty storage (first start), admin user will be created and token will be generated and print to stdout:
```
***
Admin user key: 9dca8ab2-0620-4c39-80c9-1fdbf4517ba0
***
```

Use this key to create your own users using cli, set `SD_USER` and `SD_KEY` environment variables like in example:
```
# export SD_USER=admin
# export SD_KEY=9dca8ab2-0620-4c39-80c9-1fdbf4517ba0
# ./stack-deploy adduser --name johnsnow --admin=true
User added. Key: 2c7ba33f-f5df-4d6c-8eeb-0359cd0e7577
```

To regenerate user token use `refreshtoken` command:
```
# ./stack-deploy refreshtoken --name johnsnow
New key generated: 5e1c13a6-6b25-4497-b9b4-eafea911a016
```

Please note that only admin users can create other users and refresh their tokens.

Running with Marathon
----------------------------

`marathon.json` file contains a Marathon template that is able to run stack-deploy server and bootstrap it. You will probably want to change it a little bit before using (at least replace artifact URLs and Mesos Master/Marathon address).

Use the following command to put this json template to Marathon:

    # curl -X PUT -d@marathon.json -H "Content-Type: application/json" http://master:8080/v2/apps/stack-deploy

Usage
=====
The concept of stack
------------------------
Stack is a YAML file that contains information about Applications that should be deployed on Mesos. Every stack has a unique name that allows to distinguish stacks among each other. Stack can also extend other stack to reduce amount of configurations being set. Every stack also has one or more applications which can depend on other applications in this stack.

**Stack fields description**:

`name` - [`string`] - unique stack name. This will be used to run the stack or when extending it.    
`from` - [`string`] - parent stack name to inherit configurations from. Not allowed in bootstrap stacks.    
`applications` - [`map[string]Application`] - applications to be deployed on Mesos. Map keys are application names to allow overriding configurations for specific applications and values are the actual application configurations.

**Application fields description**:

`type` - [`string`] - application type which is used to determine a `TaskRunner` to run the actual application tasks.    
`id` - [`string`] - unique application ID that will be used as in Marathon.    
`version` - [`string`] - optional field to specify an application version.    
`cpu` - [`double`] - amount of CPUs for application scheduler.    
`mem` - [`double`] - amount of memory for application scheduler.    
`ports` - [`array[int]`] - ports to accept for application scheduler. Can be left empty to accept any offered port.    
`instances` - [`string`] - number of application instances to run. Valid is any number between 1 to number of slaves, or `all` to run on all available slaves. Defaults to `1`.    
`constraints` - [`array[array[string]]`] - Marathon application constraints. Detailed info [here](https://github.com/mesosphere/marathon/blob/master/docs/docs/constraints.md).    
`user` - [`string`] - Mesos user that will be used to run the application. Defaults to current system user if not set.    
`healthcheck` - [`string`] - URL to perform healthchecks. Optional but highly recommended.    
`launch_command` - [`string`] - launch command for application scheduler. The scheduler flags should not be set here.    
`args` - [`array[string]`] - Arbitrary arguments to pass to the application.    
`env` - [`map[string]string`] - Key-value pairs to pass as environment variables.    
`artifact_urls` - [`array[string]`] - artifacts to be downloaded before running the application scheduler.    
`additional_artifacts` - [`array[string]`] - additional artifacts to be downloaded before running the application scheduler. This can be used to avoid overriding the artifact list in child stacks. All additional artifacts will be appended to artifact urls list.    
`scheduler` - [`map[string]string`] - scheduler configurations. Everything specified in these configurations will be appended to `launch_command` in form `--k v`.    
`tasks` - [`map[string]map[string]string`] - ordered map of task configurations. Map length defines the number of separate tasks launched for this application. It is up to `TaskRunner` to decide how to use information contained in each task configuration.    
`dependencies` - [`array[string]`] - application dependencies. The application in stack won't be run until all its dependencies are satisfied. E.g. applications without dependencies will be launched first, then others with resolved dependencies.    
`before_scheduler` - [`array[string]`] - any number of arbitrary shell commands to run in stack-deploy sandbox before running `launch_command`.     
`after_scheduler` - [`array[string]`] - any number of arbitrary shell commands to run in stack-deploy sandbox after running `launch_command`.     
`before_task` - [`array[string]`] - any number of arbitrary shell commands to run in stack-deploy sandbox before running each task in `tasks`.     
`after_task` - [`array[string]`] - any number of arbitrary shell commands to run in stack-deploy sandbox after running each task in `tasks`.     
`after_tasks` - [`array[string]`] - any number of arbitrary shell commands to run in stack-deploy sandbox after running all tasks in `tasks`.     

Stack inheritance
---------------------

Stacks support inheritance (the only exception are stacks used for bootstrapping as we cannot fetch parent stacks from Cassandra during bootstrapping). This way you can define a stack with some common applications/behavior (not necessarily complete) and extend it in other stacks.

Consider a following example:

```
name: default.kafka-mesos
applications:
  kafka-mesos:
    type: "kafka-mesos-0.9.x"
    id: kafka-mesos
    version: 0.9.2
    cpu: 0.5
    mem: 512 
    launch_command: "/usr/bin/java -jar kafka-mesos-0.9.2.0.jar scheduler"
    artifact_urls: 
      - "https://github.com/mesos/kafka/releases/download/v0.9.2.0/kafka-mesos-0.9.2.0.jar"
      - "http://apache.volia.net/kafka/0.8.2.2/kafka_2.10-0.8.2.2.tgz"
    healthcheck: /health
    scheduler:
      api: http://$HOST:$PORT0
      master: zk://zookeeper:2181/mesos
      zk: zookeeper:2181
      debug: true
```

```
name: dev.kafka-mesos
from: default.kafka-mesos
applications:
  kafka-mesos:
    cpu: 1
    mem: 1024
    tasks:
      brokers:
        id: 0..3
        constraints: "hostname=unique"
```

The `dev.kafka-mesos` stack will inherit everything contained in `default.kafka-mesos` but will also override `cpu` and `mem` and add `tasks`. It is also possible for some other stack to extend `dev.kafka-mesos` - there are no limitations on how many inheritance layers you have.

Adding stacks
----------------

All stacks should be kept in stack-deploy before being run. To add a stack define a stack file and run the following command:

    # ./stack-deploy add --file path/to/stackfile

You can also pass an `--api` flag to specify the address of stack-deploy server. By default it assumes stack-deploy is running on `127.0.0.1:4200`.

Available flags:

`--api` - [`http://127.0.0.1:4200`] - stack-deploy server address.    
`--file` - [`Stackfile`] - Stackfile with application configurations.    
`--debug` - [`false`] - Flag for debug mode.

Removing stacks
--------------------

To remove a stack run the following command:

    # ./stack-deploy remove dev.kafka-mesos --zone devzone

If the given stack does not have any dependent stacks it will be removed. Otherwise you will get an error message with a list of dependent stacks that should be removed first. Alternatively you may pass a `--force` flag to remove a stack and all its children. Be careful though as it doesn't ask for confirmation.
You can also pass an `--api` flag to specify the address of stack-deploy server. By default it assumes stack-deploy is running on `127.0.0.1:4200`.

Available flags:

`--api` - [`http://127.0.0.1:4200`] - stack-deploy server address.     
`--force` - [`false`] - Flag to force delete the stack with all children.    
`--debug` - [`false`] - Flag for debug mode.

Listing stacks
----------------

To list all available stacks kept in stack-deploy run the following command:

    # ./stack-deploy list

You can also pass an `--api` flag to specify the address of stack-deploy server. By default it assumes stack-deploy is running on `127.0.0.1:4200`.

Available flags:

`--api` - [`http://127.0.0.1:4200`] - stack-deploy server address.    
`--debug` - [`false`] - Flag for debug mode.

Showing stacks
------------------

To show stack contents run the following command:

    # ./stack-deploy show dev.kafka-mesos

You can also pass an `--api` flag to specify the address of stack-deploy server. By default it assumes stack-deploy is running on `127.0.0.1:4200`.

Available flags:

`--api` - [`http://127.0.0.1:4200`] - stack-deploy server address.    
`--debug` - [`false`] - Flag for debug mode.

Running stacks
-----------------

To run a stack run the following command:

    # ./stack-deploy run dev.kafka-mesos --zone us-east-1

This call will block until the stack successfully deploys or an error occurs.
You can also pass an `--api` flag to specify the address of stack-deploy server. By default it assumes stack-deploy is running on `127.0.0.1:4200`.

Available flags:

`--var` - arbitrary variables to add to stack context. Multiple occurrences of this flag allowed.    
`--skip` - regular expression of applications to skip in stack. Multiple occurrences of this flag allowed.    
`--zone` - [empty string] - zone to run stack in.    
`--max.wait` - [`600`] - maximum time in seconds to wait for each application in a stack to become running and healthy.    
`--api` - [`http://127.0.0.1:4200`] - stack-deploy server address.    

Running Docker containers
----------------------------

Stack-deploy uses Marathon to run Docker containers. A simple example of a task that runs a Docker container is as follows:

```
name: docker
applications:
  docker:
    type: "docker"
    id: docker-task
    cpu: 0.1
    mem: 128
    launch_command: "sleep 60 && echo done!"
    docker:
      image: busybox
```

**Docker fields description**:

`force_pull_image` - [`bool`] - force pull Docker image. Defaults to `false`.    
`image` - [`string`] - Docker image to run.    
`network` - [`string`] - Network mode to use.    
`parameters` - [`map[string]string`] - Additional key-value parameters.    
`port_mappings` - Port mappings for bridged networking.    
`privileged` - [`bool`] - Flag to run the Docker container in privileged mode. Defaults to `false`.    
`volumes` - Additional volumes.    

Run-once tasks
-----------------

Run-once tasks are a special type (for now) of tasks which are run without using Marathon. In future, potentially, stack-deploy will stop depending on Marathon at all, but that's a long path.

Run-once tasks are run exactly once and are not restarted after termination unlike Marathon tasks. This is useful for installing some additional software on slaves, running builds etc.

The number of run-once tasks is controlled using `instances` field in each application, which can be any number between 1 and number of slaves, or `all` meaning "number of slaves" instances. Run-once tasks are also compatible with [Marathon constraints](https://github.com/mesosphere/marathon/blob/master/docs/docs/constraints.md). You may control where tasks will run. The "all" number will be also adjusted to the number of matching slaves, so if you have for example 9 slaves, 6 with attribute type=kafka and 3 with type=zk, and your run-once task is constrained like `["type", "LIKE", "kafka"]`, then `all` instances will equal to 6.

An example of a stack with run once task could be the following:

```
name: test.run.once
applications:
  test-run-once:
    type: "run-once" #this is a special application type that should be used for run-once tasks.
    id: test-run-once
    cpu: 0.1 # CPUs and memory for each instance of a task
    mem: 512
    instances: all #Run on all available slaves
    launch_command: "sleep 30 && echo done!"
```

A run-once application is considered successful if all its tasks returned with status code 0. If any task returns with non-zero exit code, the whole application is considered failed and tasks are not retried. Stack deployment then stops immediately.

Minimal stack examples
----------------------------

Minimal stack examples are located in `stacks` directory in this repository.

Developer mode
------------------

Stack-deploy supports running the server in developer mode. **DO NOT** use this mode in production. Differences in developer mode are the following:

1. Cassandra is not required - all stacks will be kept in memory. Thus failovers will lead to stack loss.    
2. No authentication required - `SD_USER` and `SD_KEY` are ignored.    
3. Mesos failover timeout set to 0 - framework will unregister and kill all run-once tasks once stack-deploy server shuts down.

Example Usage
-------------
```
./stack-deploy addlayer --file stacks/cassandra_dc.stack --level datacenter
./stack-deploy addlayer --file stacks/cassandra_cluster.stack --level cluster --parent cassandra_dc
./stack-deploy addlayer --file stacks/cassandra_zone1.stack --level zone --parent cassandra_cluster
./stack-deploy addlayer --file stacks/cassandra_zone2.stack --level zone --parent cassandra_cluster
./stack-deploy add --file stacks/cassandra.stack
./stack-deploy run cassandra --zone cassandra_zone1
```


API
----

### Healthcheck
```
GET /health
```
Simple healthcheck, returns HTTP status 200.

### Authentication
All methods except healthcheck require authentication headers: `X-Api-User` and `X-Api-Key`.

### Stacks
```
  POST /list
  POST /get
  POST /run
  POST /createstack
  POST /removestack
```

### Users
To manage users you have to be an admin
```
  POST /createuser
  POST /refreshtoken
```

### Layers
Stack-deploy has 3-leveled layers system. You can create zones, clusters and data centers. Then you can launch you stacks in different zones.
```
POST /createlayer
{
  "layer": "zone", // "zone", "cluster" or "datacenter"
  "parent": "devcluster", // name of cluster to create zone in
  "stackfile": "<zone options here>", // zone-specifig application settings
}
```
