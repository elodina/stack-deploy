#!/bin/sh

docker-compose kill -s SIGTERM
docker-compose rm -f
GOOS=linux go build
docker-compose build
docker-compose up -d
go build
