set -ex

mkdir logs
docker ps -a >& ./log/ps.log
for name in $(docker ps --format '{{.Names}}'); do
  docker logs $name >& ./logs/$name.log
done
