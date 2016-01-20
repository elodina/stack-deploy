stealthly/go_kafka_client task runner
======================================

Implemented to run [stealthly/go_kafka_client](https://github.com/stealthly/go_kafka_client/tree/consumer-task).
Exposes stack variables:

- `$app.host` - stealthly/go_kafka_client scheduler address.
- `$app.port` - stealthly/go_kafka_client scheduler port.
- `$app.api` - stealthly/go_kafka_client scheduler api address in a form `http://$host:$port`.

For `$app` == `gokafka` variables would be:
-  `gokafka.host`, `gokafka.port` and `gokafka.api` for scheduler.