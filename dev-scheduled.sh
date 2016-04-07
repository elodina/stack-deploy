#!/bin/sh

./dev.sh
stack-deploy ping
stack-deploy add --file stacks/scheduled.stack
sleep 5
stack-deploy run scheduled
