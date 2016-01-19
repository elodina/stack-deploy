elodina/datastax-enterprise-mesos task runner
======================================

Implemented to run [elodina/datastax-enterprise-mesos](https://github.com/elodina/datastax-enterprise-mesos).
Exposes stack variables:

- `$app.host` - elodina/datastax-enterprise-mesos scheduler address.
- `$app.port` - elodina/datastax-enterprise-mesos scheduler port.
- `$app.api` - elodina/datastax-enterprise-mesos scheduler api address in a form `http://$host:$port`.
- `$app.cassandra-$id` - cassandra node CQL endpoint in a form `$host:$port`
- `$app.cassandraConnect` - comma separated connection string for all known Cassandra nodes.

For `$app` == `dse` that has Cassandra nodes `0` and `1` variables would be:
-  `dse.host`, `dse.port` and `dse.api` for scheduler.
- `dse.cassandra-0` and `dse.cassandra-1`
- `dse.cassandraConnect`