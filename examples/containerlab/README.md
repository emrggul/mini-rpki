# Add MiniRPKI to an existing Containerlab topology

This example is for an existing Containerlab user. It adds one MiniRPKI
container and one dedicated point-to-point RTR link. MiniRPKI is managed from
its own interactive shell; the router reaches its RTR listener over the data
link.

Replace `YOUR-ROUTER` and the example `192.0.2.0/30` addresses with values
from your lab plan. Do not use the management address as the RTR endpoint.

## 1. Add this node and link

After publishing a release, replace `OWNER` and `VERSION` with the GitHub
owner and release tag, for example `v0.1.0`.

```yaml
topology:
  nodes:
    mini-rpki:
      kind: linux
      image: ghcr.io/OWNER/mini-rpki:VERSION
      env:
        MINIRPKI_ADDRESS: 192.0.2.1/30
        MINIRPKI_PORT: "8282"
      binds:
        - ./minirpki-data:/data

  links:
    - endpoints: ["mini-rpki:eth1", "YOUR-ROUTER:ethN"]
```

Containerlab creates `eth1` in the MiniRPKI container for this link. When
`MINIRPKI_ADDRESS` is set, the universal MiniRPKI image waits for that
interface, assigns the address, then starts the RTR server at that IP and
`MINIRPKI_PORT`. The `/data` bind mount preserves VRPs when
the topology is destroyed and redeployed.

Use a `/30` or `/31` point-to-point IPv4 network, or use an IPv6 address plan
instead. The listener must use an address assigned to the MiniRPKI data
interface. The router side must be on the same link or have valid routing to
that address.

## 2. Configure the router

On the router interface connected to MiniRPKI, configure the other address:

```text
interface ethN
 ip address 192.0.2.2/30
!
rpki
 rpki polling_period 5
 rpki cache tcp 192.0.2.1 8282 source 192.0.2.2 preference 1
!
```

The exact syntax is FRRouting syntax. Other router vendors have different RTR
configuration commands. Keep the RTR source address explicit where the router
supports it, so it does not accidentally use its management interface.

## 3. Deploy and manage it

```sh
containerlab deploy -t your-topology.clab.yml
```

In the Containerlab UI, select the `mini-rpki` node and choose **Attach Shell**.
Then use the application commands inside that shell:

```sh
mini-rpki add 192.0.2.0/24 --asn 65001
mini-rpki list
mini-rpki status
mini-rpki interface
```

`interface` should show `192.0.2.1:8282`. On FRR, verify both the RTR session
and an actual route result:

```text
show rpki cache-connection
show rpki prefix-table
show bgp ipv4 unicast 192.0.2.0/24
```

A connected TCP cache alone is not sufficient: the prefix table and detailed
BGP route should show that the router received VRPs and calculated its own
validation state.

## Important limits

This is a laboratory cache. RTR is clear-text and unauthenticated, and
MiniRPKI does not validate real ROAs or download RPKI repositories. Use only
on trusted test networks and do not connect it to production routers.
