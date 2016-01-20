elodina/zipkin-mesos-framework task runner
======================================

Implemented to run [elodina/zipkin-mesos-framework](https://github.com/elodina/zipkin-mesos-framework).
Exposes stack variables:

- `$app.host` - elodina/zipkin-mesos-framework scheduler address.
- `$app.port` - elodina/zipkin-mesos-framework scheduler port.
- `$app.api` - elodina/zipkin-mesos-framework scheduler api address in a form `http://$host:$port`.
- `$app.query-$id.endpoint` - Zipkin Query connection endpoint in a form `$host:$port`

For `$app` == `zipkin` that has Query instance `0` variables would be:
-  `zipkin.host`, `zipkin.port` and `zipkin.api` for scheduler.
- `zipkin.query-0.endpoint`