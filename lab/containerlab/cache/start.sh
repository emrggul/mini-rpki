#!/bin/sh
set -eu

# The data interface is deliberately fixed. Containerlab creates eth1 for the
# first data link in the documented node/link pattern. Users choose only the
# address plan and RTR TCP port.
iface=eth1
: "${MINIRPKI_ADDRESS:?set MINIRPKI_ADDRESS, for example 192.0.2.1/30}"
port=${MINIRPKI_PORT:-8282}
case "$port" in
  ''|*[!0-9]*) echo "MINIRPKI_PORT must be a TCP port number" >&2; exit 1 ;;
esac
if [ "$port" -lt 1 ] || [ "$port" -gt 65535 ]; then
  echo "MINIRPKI_PORT must be between 1 and 65535" >&2
  exit 1
fi
host=${MINIRPKI_ADDRESS%/*}
if [ "$host" = "$MINIRPKI_ADDRESS" ]; then
  echo "MINIRPKI_ADDRESS must include a prefix length, for example 192.0.2.1/30" >&2
  exit 1
fi
case "$host" in
  *:*) listen="[$host]:$port" ;;
  *) listen="$host:$port" ;;
esac

# Containerlab attaches data interfaces after starting the container.
attempt=0
until ip link show "$iface" >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -gt 60 ]; then echo "containerlab interface $iface did not appear" >&2; exit 1; fi
  sleep 1
done
ip address replace "$MINIRPKI_ADDRESS" dev "$iface"
ip link set "$iface" up
chown 10001:10001 /data
mkdir -p /run/minirpki
chown 10001:10001 /run/minirpki
chmod 0700 /run/minirpki
# This entrypoint runs before the sole server process in this container.
# Docker stop/start preserves the writable layer, including an unclean socket.
rm -f /run/minirpki/control.sock
# Configure the lab interface as root, then run the application unprivileged.
exec su-exec 10001:10001 mini-rpki serve --listen "$listen"
