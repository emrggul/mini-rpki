#!/bin/sh
set -eu
mkdir -p /run/frr
chown frr:frr /run/frr
/usr/lib/frr/frrinit.sh start
trap '/usr/lib/frr/frrinit.sh stop; exit 0' TERM INT
while kill -0 "$(cat /run/frr/watchfrr.pid)" 2>/dev/null; do
  sleep 2 & wait $! || true
done
exit 1
