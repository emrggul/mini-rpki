# Verification report

Executed on 19 October 2026 using Windows amd64 with Docker Desktop 28.5.1, Linux/WSL2 kernel 6.6.87.2, Go 1.26.1, and Debian FRRouting 10.3 (`4cd62070abca`). Tests used the actual packaged RPKI module (`frr-rpki-rtrlib`).

## Current interface: direct record commands

The user-facing scenario CLI and control action were removed. Create a record with
`mini-rpki add 192.0.2.0/24 --asn 65001`; maxlen defaults to the prefix length.
Use `remove`, `list`, `clear`, and `status` directly. Optional `--maxlen`
and `--json` remain supported. The earlier vrp-prefixed commands remain
compatibility aliases. Scenario data under internal/scenario is private test
input only and is not linked into the application.

The Go unit suite, race detector, and vet passed after this change. Tests cover
default maximum lengths for IPv4 and IPv6, custom maximum lengths, exact
deletion parsing, invalid input, and rejection of the removed commands.
CLI/control response tests cover concise record acknowledgements and JSON.

The current containerlab integration test passed using only clear/add/remove commands and inspecting
FRR's real IPv4/IPv6 validation fields. Its output is saved at
`/home/emre/src/minirpki-simple-cli-test.log`. JSON-mode comparison confirmed the original two records were restored. The lab's original record snapshot
is `/home/emre/src/minirpki-before-simple-cli.json`.
Earlier sections below record previous development iterations.

## Passed

| Check | Result |
| --- | --- |
| `go test ./...` on Windows | All five packages passed |
| `go test ./...` in the Docker Linux build | All five packages passed |
| `go test -race ./...` in a Linux compiler container | All five packages passed; no race reports |
| `go vet ./...` in Linux | Passed |
| Windows and Linux/amd64 executable compilation | Passed |
| Compose configuration validation | Passed |
| WSL Ubuntu-22.04 native Docker Engine 27.5.1 / Compose 5.1.4 | Images built, Go build-stage tests passed, and POSIX FRR integration driver passed |
| Containerlab 0.76.1 on WSL | Dedicated RTR/BGP data links, IPv4 Valid/Invalid/Not Found, IPv6 Valid, and live scenario transitions passed |
| Multi-stage runtime and FRR image builds | Passed |
| Non-root runtime, local control socket, health check | Passed in Docker |
| Standalone `docker exec` add/list/remove/status | Passed |
| JSON persistence across `docker restart` | Added record retained and subsequently removed |
| FRR restart recovery | Cache restart followed by a new full synchronization; route returned to Valid |

Go tests cover IPv4/IPv6 golden wire bytes, decoding, one-byte reads/writes, invalid lengths, truncated input, exact malformed-input encapsulation, Reset/Serial exchanges, full synchronization, live announcements, withdrawals, empty deltas, unavailable history, changed sessions, version rejection, unexpected versions, bad query lengths/types, avoiding error-report loops, concurrent clients, serial wrap, bounded history, VRP validation, duplicate handling, exact deletion, atomic persistence failure, empty-dataset persistence, CLI validation, real Unix-socket control requests, concurrent store mutations, and all five scenarios. Notification timing tests exercise the one-minute boundary without shortening the production timer.

## FRR interoperability

The automated PowerShell driver (`lab/test.ps1`) inspects the explicit `rpki validation-state` field from actual per-prefix BGP output. The router received both IPv4 and IPv6 VRPs, not merely a TCP connection.

| Advertised route / action | Router result |
| --- | --- |
| 192.0.2.0/24, AS65001, demonstration VRPs | Valid |
| 192.0.2.0/25, AS65001, maxlen 24 | Invalid |
| 198.51.100.0/24, AS65001, no covering VRP | Not Found |
| 2001:db8:100::/48, AS65001 | Valid |
| Apply `invalid-asn` while connected | 192.0.2.0/24 became Invalid |
| Apply `not-found` while connected | 192.0.2.0/24 became Not Found |
| Restore `valid` while connected | 192.0.2.0/24 became Valid |

The cache container was not replaced during scenario transitions. Logs show withdrawals/announcements and matching End of Data serials. One measured change committed at 20:58:58 UTC and was fetched by FRR at 20:59:28 UTC. Only the first change in that interval triggered Serial Notify, demonstrating why server commits do not imply immediate router-side validation.

## Problems found and resolved

- The initial dual-AFI BGP configuration failed because the Docker network had no IPv6 addresses. Enabling dual-stack networking established the session and allowed actual IPv6 route validation.
- An initial integration assertion treated a BGP route header as proof of the requested state. FRR prints that header even with an unmatched state filter. Both drivers now inspect explicit validation-state fields; the corrected PowerShell test was rerun.
- Malformed input was initially reconstructed when encapsulated in an Error Report. The codec now preserves received bytes verbatim, with regression coverage.
- Go was absent from PATH and the default cache directory was not writable. A workspace-local toolchain/cache enabled native tests. Windows race builds lacked GCC; Linux container race tests succeeded instead.

## Unexecuted and remaining limits

- Both drivers have now been executed: PowerShell against Docker Desktop, and `sh lab/test.sh` against Ubuntu-22.04's native Docker Engine under WSL2. Both passed explicit IPv4/IPv6 router-state assertions and live scenario transitions. The WSL test log is `/home/emre/src/minirpki-lab-test.log`.
- Direct execution of the standalone Windows binary was blocked by the host's Application Control policy. Windows Go test executables ran successfully, and the Linux container executable and CLI were exercised.
- No Nokia, IOS XR, Junos, other FRR releases, ARM runtime, long-duration load, power-loss filesystem recovery, or protocol fuzzing campaign was tested.
- No remaining failure is known in the tested Linux/Docker/FRR path. Version 0, Router Keys, encrypted transport, real ROA validation, and automatic BGP provisioning are intentionally unsupported. See the README for stale-socket recovery and trusted-lab security boundaries.
- Images use fixed release tags, while Debian package repositories receive updates. Reproducing the exact package build later requires retaining the built images or using a Debian snapshot mirror.

The active lab now uses containerlab and is left running with the valid scenario restored. Stop it with `./lab/containerlab/lab down`. The earlier WSL Compose and Docker Desktop deployments have been stopped. The temporary standalone smoke container was removed.

The active source is `/home/emre/src/MiniRPKI`; the Windows copy was removed at the user's request after checksum comparison. The containerlab deployment uses Ubuntu's Docker Engine. VRPs persist in `lab/containerlab/data/vrps.json`. The earlier Compose `lab_rpki-data` volume was retained. The `./lab/lab` Compose helper and `./lab/containerlab/lab` helper were exercised against their respective deployments. See [the containerlab walkthrough](containerlab.md).

Containerlab's integration log is `/home/emre/src/minirpki-containerlab-test.log`. Assertions inspect explicit per-route validation fields and compare the cache container's `StartedAt` timestamp before/after the transitions, confirming no cache restart. The topology uses `kind: linux` with local images, FRR 10.3's RPKI module, and real veth links: cache eth1 ↔ router eth1, router eth2 ↔ peer eth1. The cache uses `10.82.0.1:8282`; the BGP peer uses `10.82.1.2`. The cache startup wrapper drops to UID/GID 10001 before executing MiniRPKI.

A destroy/redeploy test exposed a stale socket left by containerlab's forced container removal when the socket was in the persistent data bind mount. The lab image now sets `MINIRPKI_SOCKET=/run/minirpki/control.sock`, while JSON remains in `/data`. The lifecycle helper waits for the control interface before returning from deployment. VRP persistence and cache startup were rechecked across destroy/redeploy.

A further cold-start race let FRR initially choose its management source address before its data-interface configuration was applied. The RTR cache configuration now explicitly sets `source 10.82.0.2`. Cold-start synchronization and validation were rechecked using that source. Windows source files were removed after checksum verification; the empty Windows directory remains locked by the active host workspace.

## Readable CLI and faster refresh update

The CLI now renders aligned VRP/scenario tables and labeled cache/client status.
An explicit `--json` option retains structured output for scripts. Unit tests
cover JSON semantic preservation, empty datasets, scenario expectations, and
invalid responses. Status reports the advertised RTR timers.

Default RTR refresh is now five seconds. `--refresh` / `MINIRPKI_REFRESH` accepts
1..599 seconds; retry/expire remain 10/600. Tests cover a complete one-second
End of Data exchange and reject values that conflict with the fixed expire
interval. Serial Notify remains rate-limited to one per minute.

Executed after the change: all five Go packages passed `go test ./...`,
`go test -race ./...`, and `go vet ./...` in Linux. Updated application and lab
images built successfully. The full containerlab IPv4/IPv6 validation test and
live scenario transitions passed. The original two VRPs were retained.

Consecutive measured FRR changes (wall time from CLI request to the actual
per-route `rpki validation-state` field):

- invalid-asn ? invalid: 4.315 seconds.
- not-found ? not found: 4.67 seconds.
- valid ? valid: 5.056 seconds.

Measurements are observations from this local lab, not universal update-time
guarantees. The measurement checked the cache's StartedAt timestamp and
confirmed no restart. Results are also saved to
`/home/emre/src/minirpki-refresh-results.json`. Integration output is
`/home/emre/src/minirpki-readable-fast-test.log`.

Redeployment revealed that an explicit FRR RTR source address alone did not
fully prevent a cold-start management-path connection. A lab-specific startup
wrapper now waits for data interfaces and configures addresses from the mounted
FRR fixture before starting its daemons. Actual RTR sessions then used
10.82.0.2 and both address families converged.


## Interactive shell help correction

Plain help inside the container originally invoked Alpine ash's built-in help.
The image now loads an interactive shell initialization file through ENV, makes
help invoke mini-rpki --help, and displays a welcome message explaining the
mini-rpki command prefix. Direct application commands and noninteractive server
startup are unchanged.

Validated in the deployed container with a real allocated TTY: the welcome
message appeared and plain help displayed MiniRPKI commands instead of shell
built-ins. A disposable server accepted help, add, list, status, remove, and
clear. Its default serve command also started successfully. JSON comparison
confirmed the live user records were preserved across redeployment. The Docker
image build passed; no Go source changed for this shell-only correction.


## RTR address command

Added mini-rpki address (and --json) to show the running listener and its matching
container interfaces. The prepared deployment reports 10.82.0.1 TCP/8282 on eth1,
without advertising the non-listening management address. Wildcard listeners
list matching interface addresses. The output explicitly distinguishes
container bindings from external routing/NAT reachability.

All Go packages passed unit tests, race detection, and vet. Tests cover the
real dynamically assigned listener port, not-ready behavior, wildcard family
filtering, command parsing, and readable output. After deployment the live
command matched 10.82.0.1:8282, JSON comparison confirmed user records unchanged,
and FRR reestablished its RTR session. No external port publishing or routing
changes were made.

The user-facing endpoint command was renamed to mini-rpki interface, including JSON mode, help, lab helpers, docs, and CLI tests. Its internal control action remains stable for compatibility. The image build and unit suite passed. The live CLI was updated atomically; interface reported 10.82.0.1:8282 and records/server StartedAt were unchanged.
