# Security policy

MiniRPKI is a laboratory RTR cache, not a production RPKI validator. Its RTR
service is clear-text and unauthenticated by design. Run it only on trusted
test networks and never use its operator-defined VRPs as production routing
authorization.

## Reporting a vulnerability

Do not disclose a suspected vulnerability in a public issue. Use GitHub's
private vulnerability-reporting feature for this repository once it is enabled.
Until then, contact the repository owner through their GitHub profile and
include a minimal reproduction, affected version or commit, and impact.

Please do not include credentials, private topology data, or production route
information in a report.

## Security boundaries and known limits

- The local management API is HTTP framing over a mode-0600 Unix socket; it is
  not a network API.
- The RTR TCP listener has no TLS or client authentication.
- RTR PDU input is bounded to 4096 bytes; management request bodies are
  bounded to 4096 bytes.
- The project has no connection-count quota, so place the listener on a trusted
  laboratory network.
- MiniRPKI does not download repositories or validate signed RPKI objects.

Supported behavior is the current default branch and the latest tagged release.
