#!/bin/sh
set -eu
cd "$(dirname "$0")/../.."
lab=./lab/containerlab/lab
wait_route() {
  prefix=$1 state=$2 family=${3:-ipv4}
  label=$state
  if [ "$state" = notfound ]; then label='not found'; fi
  for attempt in $(seq 1 90); do
    if $lab router -c "show bgp $family unicast $prefix" | grep -F "rpki validation-state: $label" >/dev/null; then
      echo "PASS: $prefix $state"
      return
    fi
    sleep 2
  done
  $lab router -c 'show rpki cache-connection' -c 'show rpki prefix-table' -c "show bgp $family unicast"
  echo "FAIL: $prefix did not become $state" >&2
  exit 1
}
before=$(docker inspect --format '{{.State.StartedAt}}' clab-minirpki-mini-rpki)
$lab clear
$lab add 192.0.2.0/24 --asn 65001
$lab add 2001:db8:100::/48 --asn 65001
wait_route 192.0.2.0/24 valid
wait_route 192.0.2.0/25 invalid
wait_route 198.51.100.0/24 notfound
wait_route 2001:db8:100::/48 valid ipv6
$lab router -c 'show rpki cache-connection' -c 'show rpki prefix-table'
$lab remove 192.0.2.0/24 --asn 65001
$lab add 192.0.2.0/24 --asn 65002
wait_route 192.0.2.0/24 invalid
$lab remove 192.0.2.0/24 --asn 65002
wait_route 192.0.2.0/24 notfound
$lab add 192.0.2.0/24 --asn 65001
wait_route 192.0.2.0/24 valid
wait_route 2001:db8:100::/48 valid ipv6
after=$(docker inspect --format '{{.State.StartedAt}}' clab-minirpki-mini-rpki)
test "$before" = "$after"
echo 'PASS: containerlab data links, FRR validation states and live updates without cache restart'
