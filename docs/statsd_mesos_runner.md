elodina/statsd-mesos-kafka task runner
======================================

Implemented to run [elodina/statsd-mesos-kafka](https://github.com/elodina/statsd-mesos-kafka).
Exposes stack variables:

- `$app.host` - elodina/statsd-mesos-kafka scheduler address.
- `$app.port` - elodina/statsd-mesos-kafka scheduler port.
- `$app.api` - elodina/statsd-mesos-kafka scheduler api address in a form `http://$host:$port`.

For `$app` == `statsd` variables would be:
-  `statsd.host`, `statsd.port` and `statsd.api` for scheduler.