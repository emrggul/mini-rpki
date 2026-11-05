#!/bin/sh
# Run from repository root: sh lab/test.sh
set -eu
dc() { docker compose -f lab/compose.yaml "$@"; }
router() { dc exec -T router vtysh -c "$1"; }
wait_route() {
  prefix=$1 state=$2 family=${3:-ipv4}
  label=$state
  if [ "$state" = notfound ]; then label='not found'; fi
  i=0
  while [ "$i" -lt 90 ]; do
    if router "show bgp $family unicast $prefix" | grep -F "rpki validation-state: $label" >/dev/null; then
      echo "PASS: $prefix $state"
      return
    fi
    i=$((i+1)); sleep 2
  done
  router 'show rpki cache-connection'
  router 'show rpki prefix-table'
  router 'show bgp ipv4 unicast'
  echo "FAIL: $prefix never reached $state" >&2
  exit 1
}
dc up -d --build
before=$(dc ps -q mini-rpki)
dc exec -T mini-rpki mini-rpki clear
dc exec -T mini-rpki mini-rpki add 192.0.2.0/24 --asn 65001
dc exec -T mini-rpki mini-rpki add 2001:db8:100::/48 --asn 65001
wait_route 192.0.2.0/24 valid
wait_route 192.0.2.0/25 invalid
wait_route 198.51.100.0/24 notfound
wait_route 2001:db8:100::/48 valid ipv6
router 'show rpki cache-connection'
router 'show rpki prefix-table'
dc exec -T mini-rpki mini-rpki remove 192.0.2.0/24 --asn 65001
dc exec -T mini-rpki mini-rpki add 192.0.2.0/24 --asn 65002
wait_route 192.0.2.0/24 invalid
dc exec -T mini-rpki mini-rpki remove 192.0.2.0/24 --asn 65002
wait_route 192.0.2.0/24 notfound
dc exec -T mini-rpki mini-rpki add 192.0.2.0/24 --asn 65001
wait_route 192.0.2.0/24 valid
test "$before" = "$(dc ps -q mini-rpki)"
echo 'PASS: FRR route states and live updates without container replacement'
