elodina/exhibitor-mesos-framework task runner
======================================

Implemented to run [elodina/exhibitor-mesos-framework](https://github.com/elodina/exhibitor-mesos-framework).
Exposes stack variables:

- `$app.host` - elodina/exhibitor-mesos-framework scheduler address.
- `$app.port` - elodina/exhibitor-mesos-framework scheduler port.
- `$app.api` - elodina/exhibitor-mesos-framework scheduler api address in a form `http://$host:$port`.
- `$app.exhibitor-$id` - Zookeeper connection endpoint in a form `$host:$port`
- `$app.zkConnect` - comma separated connection string for all known Zookeeper nodes.

For `$app` == `exhibitor` that has Exhibitor nodes `0` and `1` variables would be:
-  `exhibitor.host`, `exhibitor.port` and `exhibitor.api` for scheduler.
- `exhibitor.exhibitor-0` and `exhibitor.exhibitor-1`
- `exhibitor.zkConnect`