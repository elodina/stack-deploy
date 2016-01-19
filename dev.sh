#!/bin/sh

docker-compose kill
docker-compose rm -f
GOOS=linux go build
docker-compose build
docker-compose up -d
go build
