# Development summary

MiniRPKI is implemented as one dependency-free Go executable. The protocol codec was written for this project from the RFC wire formats; no StayRTR code was incorporated. TCP handlers produce full snapshots or net differences between retained snapshots. A per-connection writer lock protects response framing and notification timing. The store protects dataset, history, and serial changes with one mutex.

Snapshot history was chosen over a stream of operations so intermediate add/remove cycles cannot produce duplicate announcements or withdrawals of records a router never received. For the intended small manual lab datasets, the memory cost is acceptable. Unknown sessions and serials fall back to a full reset exchange.

The CLI uses HTTP framing over a Unix socket, keeping management off the network while avoiding an additional protocol dependency. Persistence uses JSON, same-directory temporary files, file sync, and rename. Scenarios replace records transactionally and describe expected router outcomes; only the FRR integration driver measures actual router states.

The Docker runtime uses an unprivileged account and a statically linked executable. A separate Debian FRR image installs the RPKI module explicitly. The lab network is internal and dual-stack; the extra peer advertises actual BGP routes. The integration assertions inspect explicit per-route RPKI validation fields rather than interpreting TCP connectivity or the presence of a BGP route as success.

Initial validation uncovered a missing IPv6 lab network and an overly broad FRR output assertion. Both were corrected. Malformed-PDU error encapsulation also received a regression test to ensure the original input bytes are preserved. See [verification](verification.md) for final results and unexecuted coverage.


The CLI now has a separate human-readable renderer in
`cmd/mini-rpki/output.go`; the local control protocol stays JSON.
`--json` is available across CLI responses. RTR refresh defaults to five seconds
and can be set to 1..599 seconds; retry/expire and the notification rate limit
retain their protocol-safe values. The containerlab FRR startup wrapper applies
fixture interface addresses before launching FRR to eliminate bootstrap path
selection races. See verification for measured router update times.


The current user interface centers on direct `add`, `remove`, `list`,
`clear`, and `status` commands. Maximum length defaults to the prefix length.
Scenario commands and their control action were removed. Private validation
fixtures retain the old internal package name but are not part of the runtime.
Containerlab/Compose tests and guides now use explicit record changes.
