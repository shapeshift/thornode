#!/bin/sh

mkdir logs
docker ps -a >./logs/ps.log 2>&1
for name in $(docker ps --format '{{.Names}}'); do
  docker logs "$name" >"./logs/$name.log" 2>&1
done
