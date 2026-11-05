#!/bin/sh
set -eu

# The same image supports normal Docker and Containerlab use.
# Normal Docker use starts MiniRPKI unprivileged without network setup.
if [ "$#" -gt 0 ] && [ "$1" = "mini-rpki" ]; then
  shift
fi
if [ "$#" -eq 0 ]; then
  set -- serve
fi
if [ -z "${MINIRPKI_ADDRESS:-}" ] || [ "$1" != "serve" ]; then
  exec su-exec 10001:10001 mini-rpki "$@"
fi

# Containerlab attaches eth1 after starting a Linux node. When an address is
# supplied, configure that dedicated data link before launching the RTR server.
iface=eth1
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

attempt=0
until ip link show "$iface" >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -gt 60 ]; then
    echo "containerlab interface $iface did not appear" >&2
    exit 1
  fi
  sleep 1
done
ip address replace "$MINIRPKI_ADDRESS" dev "$iface"
ip link set "$iface" up
chown 10001:10001 /data
mkdir -p /run/minirpki
chown 10001:10001 /run/minirpki
chmod 0700 /run/minirpki
rm -f /run/minirpki/control.sock
shift
exec su-exec 10001:10001 mini-rpki serve "$@" --listen "$listen"
