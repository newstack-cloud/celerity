#!/bin/sh
# Six valkey servers in one container, each announcing 127.0.0.1 on a published port so
# a client on the host can follow a redirection to any of them. A cluster that
# announced its own addresses would be reachable only for the first hop.
set -e

PORTS="7100 7101 7102 7103 7104 7105"

for port in $PORTS; do
  valkey-server \
    --port "$port" \
    --cluster-enabled yes \
    --cluster-config-file "/tmp/nodes-$port.conf" \
    --cluster-node-timeout 5000 \
    --cluster-announce-ip 127.0.0.1 \
    --cluster-announce-port "$port" \
    --cluster-announce-bus-port "1$port" \
    --appendonly no \
    --save '' \
    --daemonize yes
done

# Waited for rather than paused on, since a machine under load can take longer than
# any fixed pause and forming the cluster would then fail outright.
deadline=$(( $(date +%s) + 30 ))
for port in $PORTS; do
  until valkey-cli -p "$port" ping 2>/dev/null | grep -q PONG; do
    if [ "$(date +%s)" -ge "$deadline" ]; then
      echo "valkey on port $port was not accepting connections after 30 seconds" >&2
      exit 1
    fi
    sleep 0.1
  done
done

# A restart brings the nodes back with the cluster they were already in, read
# from the config files they keep. Asking them to form one again is refused,
# which stopped the container rather than bringing it back up. A node that has
# never been in a cluster only knows about itself.
known_nodes=$(valkey-cli -p 7100 cluster info 2>/dev/null | tr -d '\r' \
  | sed -n 's/^cluster_known_nodes:\(.*\)$/\1/p')

if [ "$known_nodes" = "1" ]; then
  valkey-cli --cluster create \
    127.0.0.1:7100 127.0.0.1:7101 127.0.0.1:7102 \
    127.0.0.1:7103 127.0.0.1:7104 127.0.0.1:7105 \
    --cluster-replicas 1 --cluster-yes
else
  echo "the nodes are already in a cluster, waiting for them to agree on it again"
fi

# Waited for either way, since nodes that have just been given a cluster and
# nodes coming back to one both take a moment to agree on it. Reaching this
# without agreement is a cluster nothing can be read from, which is worth
# stopping for rather than reporting as ready.
deadline=$(( $(date +%s) + 30 ))
until valkey-cli -p 7100 cluster info 2>/dev/null | tr -d '\r' \
  | grep -q '^cluster_state:ok$'; do
  if [ "$(date +%s)" -ge "$deadline" ]; then
    echo "the valkey nodes did not agree on a cluster within 30 seconds" >&2
    exit 1
  fi
  sleep 0.1
done

echo "cluster ready"
tail -f /dev/null
