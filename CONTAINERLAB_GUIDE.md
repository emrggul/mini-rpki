# Deploy and use MiniRPKI with containerlab

MiniRPKI lets you test RPKI origin validation on a real FRRouting router using records you control. You run a cache, a receiving router, and a BGP peer in containers. No external validator or RPKI repository is needed.

Use this lab only on trusted test networks. These are operator-defined test VRPs, not cryptographically validated Internet authorizations. Do not connect this cache to production routers.

## What the lab contains

~~~text
MiniRPKI                 FRR router                 FRR peer
10.82.0.1 ---- RTR ---- 10.82.0.2
                        10.82.1.1 ---- BGP ---- 10.82.1.2
                        AS65000                  AS65001
~~~

The peer originates real BGP routes. MiniRPKI supplies VRPs over RTR. The receiving FRR router compares each BGP route with those VRPs.

| Result | Meaning |
| --- | --- |
| Valid | At least one covering VRP authorizes both the origin AS and route length |
| Invalid | Covering VRPs exist, but none authorizes both |
| Not Found | No VRP covers the route |

A VRP is an IP prefix, a maximum permitted route length, and an origin ASN. Adding a VRP does not advertise a BGP route.

## 1. Open WSL and check prerequisites

This project's existing environment is Ubuntu-22.04 under WSL. From Windows PowerShell:

~~~powershell
wsl -d Ubuntu-22.04
~~~

Inside WSL:

~~~sh
cd /home/emre/src/MiniRPKI
docker version
containerlab version
~~~

For a fresh machine, place this source tree in your Linux filesystem and change to its root. Install a working Docker Engine and containerlab using the [official containerlab installation instructions](https://containerlab.dev/install/). Containerlab needs Linux networking privileges; its supported setup may use sudo or its configured privilege mechanism. Docker must show a working Server section.

Keep all commands below in the same WSL Docker environment. Docker Desktop and native WSL Docker can use different engines, images, and containers.

To open the checkout in Visual Studio Code with its WSL extension, run:

~~~sh
code .
~~~

## 2. Build the images

From the repository root:

~~~sh
chmod +x lab/containerlab/lab
./lab/containerlab/lab build
~~~

This builds the application, the FRR image with its RPKI module, and a cache image that configures the containerlab data link.

The initial build needs access to base images and package repositories. Once built, normal lab operation needs no Internet connection. The topology uses local image tags and disables image pulling.

## 3. Deploy the topology

~~~sh
./lab/containerlab/lab up
./lab/containerlab/lab status
~~~

If your containerlab installation requires sudo, run the deployment helper with sudo. This project's existing WSL installation supports direct invocation.

The helper waits for MiniRPKI's control socket. BGP and RTR may need a few additional seconds to converge.

| Container | Role | Management address |
| --- | --- | --- |
| clab-minirpki-mini-rpki | RTR cache | 172.31.82.2 |
| clab-minirpki-router | Receiving FRR router | 172.31.82.3 |
| clab-minirpki-peer | BGP originator | 172.31.82.4 |

The cache's RTR endpoint is **10.82.0.1:8282**, on its data link. Its management address is not the RTR endpoint in this topology.

### Reuse the cache node with another address plan

The Containerlab cache wrapper reads these two environment values from the `mini-rpki` node:

```text
MINIRPKI_ADDRESS    interface address with prefix, for example 192.0.2.1/30
MINIRPKI_PORT       RTR TCP port, for example 8282
```

The wrapper always uses `eth1`, the first data-link interface in the documented Containerlab node/link pattern. The reference topology supplies `10.82.0.1/30` and port `8282`. In another lab, change only those two values and configure the router's connected interface and RTR cache target to match. The wrapper waits for `eth1`, assigns `MINIRPKI_ADDRESS`, then drops to the unprivileged MiniRPKI user before starting the server.

Attach a shell to the MiniRPKI node to manage records with `mini-rpki add`, `remove`, `list`, and `status`. The management path is separate from the router-to-cache data link.

If an earlier deployment exists but containers have exited, inspect their logs first:

~~~sh
docker ps -a --filter name=clab-minirpki
docker logs --tail 100 clab-minirpki-mini-rpki
docker logs --tail 100 clab-minirpki-router
~~~

If all nodes exit after about one minute with `Missing data interface eth1` or `containerlab eth1 did not appear`, the startup scripts timed out waiting for containerlab links. Starting existing containers with Docker does not recreate these links. Use containerlab deployment rather than Docker Start/Restart.

If MiniRPKI immediately exits with `control.sock: bind: address already in use` after Stop/Start, its old image retained a stale control socket. The cache startup wrapper now removes that socket before launching its sole server. Build the updated cache image with `docker build -t mini-rpki-clab:local lab/containerlab/cache`, then recreate the topology below. Building alone does not update existing containers. Containerlab Start can restore links, but an endpoint exiting during restoration can disrupt that recovery.

To recreate that topology:

~~~sh
./lab/containerlab/lab down
./lab/containerlab/lab up
~~~

Saved VRPs remain on disk. Unsaved interactive FRR configuration is lost. Prefer topology redeployment over starting individual stopped containers, because containerlab must recreate their links.

## 4. Open the router CLI

~~~sh
./lab/containerlab/lab router
~~~

You are now inside FRR vtysh. Enter these commands directly at the router prompt:

~~~text
show interface brief
show ip route
show bgp summary
show rpki configuration
show rpki cache-connection
show rpki prefix-table
show bgp ipv4 unicast
show bgp ipv6 unicast
show bgp ipv4 unicast 192.0.2.0/24
show bgp ipv4 unicast 192.0.2.0/25
show bgp ipv4 unicast 198.51.100.0/24
show bgp ipv6 unicast 2001:db8:100::/48
~~~

Look for the RPKI validation-state field in detailed BGP route output. A connected cache alone does not prove route validation works. Use exit to return to WSL.

You can open the advertising router with:

~~~sh
./lab/containerlab/lab peer
~~~

For a single noninteractive query:

~~~sh
./lab/containerlab/lab router -c 'show rpki cache-connection'
~~~

With a fresh data directory, MiniRPKI starts with two records:

| VRP | Maximum length | Origin |
| --- | --- | --- |
| 192.0.2.0/24 | 24 | AS65001 |
| 2001:db8:100::/48 | 48 | AS65001 |

For these defaults, the peer's routes should produce:

| BGP route | Origin | Expected result |
| --- | --- | --- |
| 192.0.2.0/24 | AS65001 | Valid |
| 192.0.2.0/25 | AS65001 | Invalid: longer than maximum 24 |
| 198.51.100.0/24 | AS65001 | Not Found |
| 2001:db8:100::/48 | AS65001 | Valid |

An existing saved dataset overrides the defaults. Check your actual records before expecting these results.

## 5. Change records without restarting

In the Containerlab UI, select the `mini-rpki` node and choose **Attach
Shell**. Then manage the cache inside that shell:

~~~sh
mini-rpki list
mini-rpki add 198.51.100.0/24 --asn 65001
~~~

For terminal automation, the helper offers the same application commands:

~~~sh
./lab/containerlab/lab list
./lab/containerlab/lab add 198.51.100.0/24 --asn 65001
~~~

After the next synchronization, inspect the route on FRR:

~~~sh
./lab/containerlab/lab router -c 'show bgp ipv4 unicast 198.51.100.0/24'
~~~

It should become Valid when that VRP authorizes the advertised route. Remove the exact record:

~~~sh
./lab/containerlab/lab remove 198.51.100.0/24 --asn 65001
~~~

It becomes Not Found if no other covering VRP remains.

To authorize the /25 route as well as the /24, replace the original maximum length:

~~~sh
./lab/containerlab/lab remove 192.0.2.0/24 --asn 65001 --maxlen 24
./lab/containerlab/lab add 192.0.2.0/24 --asn 65001 --maxlen 25
~~~

These are two separate updates. To restore the original setting, remove the maxlen 25 record and add the maxlen 24 record.

IPv6 uses the same syntax:

~~~sh
./lab/containerlab/lab add 2001:db8:200::/48 --asn 65001 --maxlen 64
~~~

This creates a VRP only. The provided BGP peer does not advertise this additional prefix unless you configure it.

Maximum length defaults to the prefix length. Duplicate adds do not create duplicates. Removal must match prefix, maximum length, and ASN; removing a maxlen 25 record with the default maxlen 24 will fail.

## 6. Commands available in the MiniRPKI shell

Inside the shell attached in the Containerlab UI:

~~~sh
help
mini-rpki list
mini-rpki add 198.51.100.0/24 --asn 65001
mini-rpki remove 198.51.100.0/24 --asn 65001
mini-rpki status
mini-rpki interface
exit
~~~

Use the mini-rpki prefix for commands. The shell's help function displays application help; it does not turn add or list into standalone shell commands.

| Command | Purpose |
| --- | --- |
| mini-rpki list | Show served VRPs |
| mini-rpki add PREFIX --asn ASN [--maxlen N] | Add an authorization |
| mini-rpki remove PREFIX --asn ASN [--maxlen N] | Remove one exact authorization |
| mini-rpki clear | Delete all authorizations |
| mini-rpki status | Show cache session, serial, and clients |
| mini-rpki interface | Show listener and interface addresses |
| mini-rpki --help | Show supported commands |

Use --json for machine-readable output, for example mini-rpki list --json. Management commands communicate through a Unix socket inside the container; there is no public management TCP port.

## Addresses and other routers

Run ./lab/containerlab/lab interface to see the cache listener. In this lab, the supplied FRR router reaches it at 10.82.0.1:8282.

This address belongs to a containerlab point-to-point link. A physical router or another lab cannot necessarily reach it. You must provide routing or an appropriate connection into that network and configure the RTR listener consistently. WSL NAT, host firewalls, and Docker networking also affect reachability. The interface command reports addresses; it cannot prove external reachability.

The separate standalone Docker quick start in README publishes port 8282. This containerlab topology does not publish it. Restrict any external exposure to a trusted laboratory network.

## Update timing

Default refresh and FRR polling are five seconds. Allow time for the next successful RTR update, then verify actual FRR output. Serial Notify is limited to once per minute per connection, as required by the protocol.

For a deliberate one-second experiment, set MINIRPKI_REFRESH: "1" in the mini-rpki node's env section in lab/containerlab/minirpki.clab.yml and set rpki polling_period 1 in lab/containerlab/router.conf. Redeploy with down then up. This changes polling, not the notification rate limit, and is not a guaranteed end-to-end update deadline.

## Persistence and backups

The cache saves VRPs to /data/vrps.json, backed by lab/containerlab/data/vrps.json on WSL. The serve option --data chooses that persistence file; it is not an input command for adding records.

Normal record changes are saved automatically. A saved empty dataset remains empty after restart; defaults are only for first startup. Do not edit the file while the server runs.

Before clearing records or running tests, save a readable snapshot:

~~~sh
docker exec clab-minirpki-mini-rpki mini-rpki list --json > vrps-backup.json
~~~

There is no import command in this version. Restore records with add commands. Offline file restoration requires checking the persistence format and stopping the cache first. The on-disk file is owned by the container's non-root user and may not be directly readable by your WSL user.

Stop and remove the lab with:

~~~sh
./lab/containerlab/lab down
~~~

This preserves the host data directory. To apply a changed application image, build it and redeploy; building alone does not update running containers.

## Automated interoperability check

**This test clears your current VRPs and leaves the demonstration dataset.** Use a disposable lab or arrange a backup and restoration first.

~~~sh
./lab/containerlab/lab test
~~~

The script checks actual FRR Valid, Invalid, and Not Found states, IPv6 validation, live record changes, and that the cache did not restart. Do not interpret a mere RTR connection as a passing interoperability test. Historical results are recorded in [docs/verification.md](docs/verification.md); they are separate from the result of your own run.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| Docker cannot connect | Confirm the WSL Docker daemon is running and docker version shows Server details |
| Local image missing | Run the build helper against the same Docker engine as containerlab |
| Permission denied deploying | Check your containerlab installation's privilege setup; use sudo if required |
| Existing nodes are stopped | Inspect logs, then destroy and redeploy the topology |
| RTR disconnected | Check cache logs, 10.82.0.1:8282, router source 10.82.0.2, and show rpki cache-connection |
| BGP disconnected | Check show bgp summary and addresses on the router-peer link |
| Route absent | Check the peer's advertised networks; adding a VRP does not originate BGP |
| Unexpected validation state | Inspect detailed BGP output and all covering VRPs, including ASN and maximum length |
| Updates seem delayed | Allow a polling cycle, check client/serial status and logs, then inspect the route again |
| Defaults did not return | A saved dataset overrides defaults, including an empty saved dataset |
| Cannot read vrps.json | Use mini-rpki list --json rather than weakening file permissions |
| FRR commands missing | Use the supplied image with frr-rpki-rtrlib and bgpd's -M rpki option |

Follow cache logs with ./lab/containerlab/lab logs. Press Ctrl-C to stop following logs.

## Files to customize and further reading

- lab/containerlab/minirpki.clab.yml: nodes, links, images, management network, and cache environment.
- lab/containerlab/router.conf: receiving router's interfaces, BGP, and RPKI settings.
- lab/containerlab/peer.conf: originating router's BGP networks and interfaces.
- lab/frr/Dockerfile and daemons: FRR package/module setup.
- [README](README.md): product overview and standalone Docker use.
- [Architecture](docs/architecture.md): implementation details.
- [AI knowledge base](docs/AI_KNOWLEDGE_BASE.md): contributor handoff and current design decisions.
- [Containerlab Linux nodes](https://containerlab.dev/manual/kinds/linux/) and [FRR BGP documentation](https://docs.frrouting.org/en/latest/bgp.html): upstream references.

Edit host startup files to make router configuration reproducible. Changes made only inside vtysh are not guaranteed to survive topology recreation.
