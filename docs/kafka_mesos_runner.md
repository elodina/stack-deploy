mesos/kafka task runner
====================

Implemented to run [mesos/kafka](https://github.com/mesos/kafka).
Exposes stack variables:

- `$app.host` - mesos/kafka scheduler address.
- `$app.port` - mesos/kafka scheduler port.
- `$app.api` - mesos/kafka scheduler api address in a form `http://$host:$port`.

For `$app` == `kafka` variables would be `kafka.host`, `kafka.port` and `kafka.api` respectively.