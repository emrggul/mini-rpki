# MiniRPKI knowledge base

This is a handoff for future contributors and AI agents, updated 2026-10-20. Source code and current command help remain authoritative. Historical verification is not a claim that the lab is healthy today.

## Project and user intent

MiniRPKI is a laboratory RTR cache serving operator-defined IPv4/IPv6 VRPs. It does not download repositories, validate ROA signatures, advertise BGP routes, or assign validation states to routers. The router computes Valid, Invalid, and Not Found.

The user wants to experiment with FRRouting and MiniRPKI through containerlab on WSL. Optimize for readable router commands, simple record changes, and quick setup. Avoid dashboards, accounts, databases, Kubernetes, and platform features.

The user explicitly rejected scenario menus in favor of one-line record commands. Scenario CLI/control actions have been removed; internal/scenario remains a private test fixture package. Do not bring the menus back without a new request.

## Environment and source of truth

- Active checkout: /home/emre/src/MiniRPKI, Linux user emre.
- WSL distribution: Ubuntu-22.04. From PowerShell: wsl -d Ubuntu-22.04.
- The old Windows checkout was removed at the user's request. Do not recreate it.
- Native WSL Docker Engine was observed at 27.5.1; Compose at v5.1.4; containerlab previously verified at 0.76.1.
- Docker Desktop is a separate engine. Confirm the Docker context and daemon before diagnosing missing containers/images.
- Direct containerlab invocation works through the installed setuid binary and group membership. sudo may require a password; do not change privileges.
- Host Go was previously absent. go.mod declares Go 1.24; the Docker builder uses golang:1.26.1-alpine.
- No external Go dependencies. MIT license. The module path does not establish a published GitHub repository; do not invent clone URLs.
- On 2026-10-20 all three lab containers were observed Exited (1). The cause was not investigated during documentation work. Do not describe them as currently operational.

When tools start in Windows, explicitly execute WSL commands against the Linux checkout. A large PowerShell command can exceed Windows command-line limits; split document writes into modest chunks.

## Current interface and user decisions

Commands use the mini-rpki executable:

~~~text
mini-rpki serve
mini-rpki add 192.0.2.0/24 --asn 65001
mini-rpki remove 192.0.2.0/24 --asn 65001
mini-rpki list
mini-rpki clear
mini-rpki status
mini-rpki interface
mini-rpki --help
~~~

--maxlen defaults to the prefix length for both add and remove. Removal matches the exact prefix, maximum length, and ASN tuple. The old vrp command spelling is a hidden compatibility alias. Human-readable output is the default; --json supports scripts.

The endpoint command was renamed from address to interface at the user's request. Internally the CLI maps interface to the control action address. internal/control/address.go still uses that name. Preserve the mapping when editing either side.

Inside an interactive container shell, help is overridden through docker/shell.sh and the Docker ENV startup file to print application help instead of Alpine built-ins. Application commands still require the mini-rpki prefix; bare add is not a command.

The user asked to remove this help footer; do not reintroduce it:
?Changes affect served VRPs. Routers calculate validation states on their own schedule.?

The user wants FRR show commands inside vtysh, not just wrappers that hide the router CLI.

## Code map

| Location | Responsibility |
| --- | --- |
| cmd/mini-rpki/main.go | Process composition, flags, environment, help, local control client |
| cmd/mini-rpki/commands.go | Command parsing |
| cmd/mini-rpki/output.go | Human-readable output |
| internal/rtr/codec.go | RTR PDU encoding/decoding and bounded input |
| internal/rtr/server.go | TCP clients, response serialization, notifications, timers |
| internal/vrp/store.go | VRP validation, persistence, snapshots, session and serial state |
| internal/vrp/defaults.go | First-start demonstration records |
| internal/control | HTTP over a local Unix socket; no management TCP listener |
| internal/scenario | Test fixtures, not a supported application feature |
| docker/shell.sh | Interactive shell help |
| lab/containerlab | Primary user lab and helper |
| lab/frr | FRR image including the RPKI module |
| lab/compose.yaml | Alternative Compose lab, retained |

## Cache and protocol invariants

RTR version 1 supports Reset/Serial Query, Cache Response, IPv4/IPv6 announcement and withdrawal, End of Data, Serial Notify, Cache Reset responses, and Error Reports. Version 0, Router Keys, TLS, SSH, cryptographic validation, and fault injection are unsupported.

Input PDUs are limited to 4096 bytes. The codec handles partial reads/writes and preserves malformed input bytes for Error Report encapsulation. Avoid error-report loops. Cache Reset is a response to an unavailable incremental update, not an unsolicited notification.

Shared state is mutex-protected. Each connection serializes writes so responses and notifications cannot interleave. Notifications are rate-limited to once per minute per connection, even when polling is faster. Write deadline is 10 seconds; idle read timeout is two hours. There is no connection-count cap.

The store validates canonical netip prefixes, address-family maximum lengths, and decimal ASN range 0..4294967295, including AS0. Duplicate adds are no-ops. Missing exact deletes fail. Sorted set snapshots prevent duplicate and canceled incremental changes.

Persistence writes a same-directory temporary file, syncs it, and renames it before committing the memory/serial change. There are 129 retained snapshots, allowing 128 transitions. Exact serial identity is used across uint32 wrap. Restart loads saved records but generates a new random 16-bit session and resets history; random IDs have a finite collision probability.

First-start defaults are 192.0.2.0/24 maxlen 24 AS65001 and 2001:db8:100::/48 maxlen 48 AS65001. Existing data, including an intentionally empty saved set, overrides defaults. Never assume the user's lab still contains the defaults.

Refresh defaults to 5 seconds, configurable through --refresh or MINIRPKI_REFRESH in the range 1..599. Retry is 10 seconds and expire 600 seconds. FRR polling is configured to 5 seconds. FRR may display a different local expire default; distinguish local configuration from advertised RTR intervals.

The interface command reports actual listener/interface metadata. A wildcard listener can enumerate usable addresses, but the server cannot infer host NAT, firewall policy, published ports, or end-to-end reachability.

## Containerlab topology and workflow

Run helpers from the repository root:

~~~sh
./lab/containerlab/lab build
./lab/containerlab/lab up
./lab/containerlab/lab status
./lab/containerlab/lab router
./lab/containerlab/lab peer
./lab/containerlab/lab interface
./lab/containerlab/lab down
~~~

Topology: lab/containerlab/minirpki.clab.yml. Node names are clab-minirpki-mini-rpki, clab-minirpki-router, and clab-minirpki-peer. Image pulling is disabled in the topology; build images first.

Images: mini-rpki:local is the normal application image; mini-rpki-clab:local adds link setup and privilege dropping; mini-rpki-frr:local installs Debian FRR and frr-rpki-rtrlib. The inspected local FRR packages were 10.3-3+deb13u1. Do not assume an image build today will resolve the same package revision.

| Node/link | Address |
| --- | --- |
| Cache eth1, RTR listener | 10.82.0.1/30, TCP 8282 |
| Router eth1, RTR source | 10.82.0.2/30 |
| Router eth2, BGP | 10.82.1.1/30, 2001:db8:82:1::1/64 |
| Peer eth1, BGP | 10.82.1.2/30, 2001:db8:82:1::2/64 |
| Management eth0 | Cache 172.31.82.2, router .3, peer .4, on minirpki-mgmt |

The cache binds RTR to 10.82.0.1, not its management address. The cache wrapper always configures its first Containerlab data interface (`eth1`); the reference topology supplies its address and TCP port through `MINIRPKI_ADDRESS` and `MINIRPKI_PORT`. Another lab can use a different data-link plan by changing those two values consistently with its router configuration. No RTR host port is published by this topology. The management bridge is not an Internet-isolation guarantee. Only the separate Compose lab uses an internal Docker network.

Receiver AS65000 peers with AS65001. The peer advertises 192.0.2.0/24, 192.0.2.0/25, 198.51.100.0/24, and 2001:db8:100::/48. With first-start defaults, expected states are Valid, Invalid (length), Not Found, and Valid respectively. Existing user data can change those results.

FRR uses bgpd -M rpki and the separately installed frr-rpki-rtrlib package. Router config uses rpki cache tcp 10.82.0.1 8282 source 10.82.0.2 preference 1. Inbound soft reconfiguration supports validation reevaluation.

The FRR startup script waits for links and installs configured addresses before daemons start. This fixed an observed cold-start race where RTR tried the management network. The cache wrapper similarly waits for eth1, configures its address, and drops to UID/GID10001.

Persistence: lab/containerlab/data/vrps.json is mounted under /data. It may be mode 0600 owned by UID10001. Use the CLI for normal edits. The container image sets the control socket to /run/minirpki/control.sock, not the persistent directory; this avoids stale sockets after forced teardown.

Restart policy is deliberately no. Building an image does not replace an existing container. To apply image/topology/startup-config changes, use lab down then lab up; saved VRPs remain, while unsaved FRR interactive configuration is lost. Investigate exited containers through logs before redeploying.

## Safe verification and known pitfalls

lab/containerlab/test.sh clears records, installs defaults, asserts actual FRR validation-state fields for IPv4/IPv6, exercises wrong-AS and missing-record transitions, and checks that the cache start time did not change. It leaves demonstration records behind. It is not safe to run casually against a user's customized dataset.

Back up through the running CLI:

~~~sh
docker exec clab-minirpki-mini-rpki mini-rpki list --json > vrps-backup.json
~~~

There is no import command. Restore individual records with add, or inspect the persistence format and restore a file only while the server is stopped. Do not write the persistence file while the server runs.

Historical work verified Go unit tests on Windows/Linux, Linux race detector and vet, Docker build/health/persistence, actual FRR Valid/Invalid/Not Found and IPv6 Valid results, live changes without cache restart, and containerlab destroy/redeploy. See docs/verification.md for historical details. These checks were not rerun for this documentation task.

Historical update measurements were roughly 4.3?5.1 seconds with five-second polling. They are observations, not timing guarantees. Reducing polling does not permit faster-than-RFC notifications.

No Nokia, Cisco, Juniper, ARM, sustained-load, power-loss, or comprehensive fuzz testing is claimed. Historical Windows execution encountered application-control restrictions; Linux testing was used. Avoid interpreting old logs mentioning scenario commands as current UX.

Earlier fixes worth preserving:
- Error Reports encapsulate actual malformed input.
- FRR assertions inspect explicit validation state, not merely route existence.
- IPv6 interface addresses are installed before daemon startup.
- Runtime sockets are not persisted.
- Interactive help shows commands actually supported by MiniRPKI.
- The public command is interface while its internal control action remains address.

Some earlier experiments copied compatible binaries into running containers. Running binaries, image tags, and source can therefore differ. Prefer a clean image build and topology redeployment for reproducible code verification; do not hot-patch protocol logic casually.

## Maintaining this handoff

Stop/start diagnosis on 2026-10-20: VS Code invoked containerlab stop/start, and Start logged restored links. MiniRPKI then immediately exited with a stale `/run/minirpki/control.sock` bind error; subsequent starts timed out on eth1 and the router also exited. The cache entrypoint now removes that socket before launching its sole server, since Docker retains the writable layer across stop/start. Current-turn verification: container logs and extension logs inspected, and `sh -n lab/containerlab/cache/start.sh` passed. The updated image and stop/start behavior have not yet been verified at runtime. Earlier claims that containerlab Start never restores links are too broad; distinguish it from Docker Start.

For each change, record the final behavior, why it matters, exact tests executed, failures, and remaining limitations. Update the user guide when CLI or topology changes. Keep historical evidence separate from current runtime status and never publish a user's experimental dataset as project defaults.

## Recovery investigation, 2026-10-20

Follow-up recovery: repeated Start reused the original containers and image; all three timed out on missing eth1. Backed up stopped-container data and FRR files under `/tmp/minirpki-recovery-current`, built `mini-rpki-clab:local` with socket cleanup (image ID starts `9980adcb`), and destroyed/redeployed using the helper. Explicit `containerlab stop` followed by `containerlab start` then restored all four link endpoints successfully. Verified actual FRR IPv4 Valid/Invalid/Not Found and IPv6 Not Found with the user's preserved single-record dataset. Saved VRP SHA256 before/after matched, and backed-up FRR files matched host startup files. This was a non-mutating runtime check; the destructive integration suite was not run. Do not confuse the earlier source-only fix with this newly built and deployed image.

All three old containers exited after approximately one minute because eth1 was absent. Both startup wrappers deliberately stop after waiting 60 attempts. This is consistent with Docker starting containers without restoring containerlab veth links; the exact action that removed the links was not established. Backed up persistent cache data and mounted FRR configurations under /home/emre/src/minirpki-recovery-20261020-003241, then ran the lab down/up helpers. Containerlab recreated both data links. The installed containerlab version during this recovery was 0.81.1 (newer than the earlier observation). Saved VRP bytes were checked unchanged after recovery.

Recovery verification: all three containers remained running after 86 seconds with zero restarts; MiniRPKI health was healthy. FRR showed RTR connected, IPv4/IPv6 BGP established, IPv4 /24 Valid, /25 Invalid, and 198.51.100.0/24 Not Found. IPv6 was Not Found with the preserved user dataset; no records were changed to force a default result. This was a non-mutating runtime check, not a rerun of the destructive integration suite.
