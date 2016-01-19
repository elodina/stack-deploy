stack-deploy task runners
=====================

Each stack-deploy application that has `tasks` associated with them (like mesos-kafka scheduler that launches brokers) must implement [TaskRunner interface](https://github.com/elodina/stack-deploy/blob/master/framework/task_runner.go#L22-L25). 

Task runners serve 2 purposes:
1. They know how to interact with the application to add tasks to it correctly.
2. They fill the Stack context with any information that could be necessary for dependent applications (a simple example is Exhibitor that exposes Zookeeper connection string for Kafka brokers).

To implement a TaskRunner for your application you need to implement 2 methods:

1. `FillContext(context *Context, application *Application, task marathon.Task) error` - called once right after Marathon task reports it is running. Inside this method you might want to fill the context with task information provided by Marathon, like hostname and port this task listens to.
2. `RunTask(context *Context, application *Application, task map[string]string) error` - called once for each task. Inside this method your task runner should interpret task parameters in a way that is applicable for your application and launch the task. Inside this method all stack variables for application and current task should be already resolved. You may get or add new stack variables from context if necessary. For example, in kafka-mesos task runner each task will get the scheduler api connection string, use it to add and run a broker (or multiple brokers if id is a range) and fill the context with broker addresses.

Stack-deploy encourages you to expose variables in a following form - `$application_name.$variable` for application global variables and `$application_name.$task_name.$variable` for task variables, however never checks that.

After you implement your task runner make sure to add it to the task runner map in [main.go](https://github.com/elodina/stack-deploy/blob/master/main.go#L60).
Once this is done you may deploy stacks using your task runner. To do so you should specify the rught application [type](https://github.com/elodina/stack-deploy/blob/master/stacks/bootstrap.stack#L4).

You may also refer to documentation for existing task runners to know what variables they expose:

1. [mesos/kafka](https://github.com/elodina/stack-deploy/blob/master/docs/kafka_mesos_runner.md)
2. [elodina/datastax-enterprise-mesos](https://github.com/elodina/stack-deploy/blob/master/docs/dse_mesos_runner.md)
3. [elodina/exhibitor-mesos-framework](https://github.com/elodina/stack-deploy/blob/master/docs/exhibitor_mesos_runner.md)
4. [elodina/zipkin-mesos-framework](https://github.com/elodina/stack-deploy/blob/master/docs/zipkin_mesos_runner.md)
5. [elodina/syslog-service](https://github.com/elodina/stack-deploy/blob/master/docs/syslog_mesos_runner.md)
6. [elodina/statsd-mesos-kafka](https://github.com/elodina/stack-deploy/blob/master/docs/statsd_mesos_runner.md)
7. [stealthly/go_kafka_client](https://github.com/elodina/stack-deploy/blob/master/docs/go_kafka_client_runner.md)