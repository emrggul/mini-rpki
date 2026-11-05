#!/bin/sh
set -eu
# Containerlab attaches data links after starting containers. Configure the
# addresses from the same FRR fixture before its RTR/BGP clients can connect.
awk '
 /^interface / { iface=$2 }
 /^ ip address / { print iface, 4, $3 }
 /^ ipv6 address / { print iface, 6, $3 }
' /etc/frr/frr.conf |
while read -r iface family prefix; do
  attempt=0
  until ip link show "$iface" >/dev/null 2>&1; do
    attempt=$((attempt + 1))
    if [ "$attempt" -gt 60 ]; then echo "Missing data interface $iface" >&2; exit 1; fi
    sleep 1
  done
  ip link set "$iface" up
  if [ "$family" = 6 ]; then
    ip -6 address replace "$prefix" dev "$iface" nodad
  else
    ip address replace "$prefix" dev "$iface"
  fi
done
exec sh /start.sh
