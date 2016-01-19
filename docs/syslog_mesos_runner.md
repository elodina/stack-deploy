elodina/syslog-service task runner
======================================

Implemented to run [elodina/syslog-service](https://github.com/elodina/syslog-service).
Exposes stack variables:

- `$app.host` - elodina/syslog-service scheduler address.
- `$app.port` - elodina/syslog-service scheduler port.
- `$app.api` - elodina/syslog-service scheduler api address in a form `http://$host:$port`.

For `$app` == `syslog` variables would be:
-  `syslog.host`, `syslog.port` and `syslog.api` for scheduler.