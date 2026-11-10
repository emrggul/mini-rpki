# Quick Installation with Containerlab

MiniRPKI is a laboratory RTR cache. You enter test VRPs and your router uses
them to calculate Valid, Invalid, or Not Found. It does not download RPKI
repositories or validate real ROAs. Use it only in a trusted lab.

## 1. Install prerequisites

You need Linux, Docker Engine, and Containerlab. Install Docker and
Containerlab from their official documentation, then check that both work:

```sh
docker version
containerlab version
```

## 2. Download the image

The current release is `v0.9`:

```sh
docker pull ghcr.io/emrggul/mini-rpki:v0.9
```

This downloads a ready-to-run image. You do not need Go, source code, or an
application build step.

## 3. Add MiniRPKI to your topology for your Containerlab

Add this node and link to your existing Containerlab YAML. Replace the
addresses, port, router name, and router interface with your own values.

From the directory containing your topology YAML, create the persistent data
directory before deployment:

```sh
mkdir -p minirpki-data
```

```yaml
topology:
  nodes:
    mini-rpki:
      kind: linux
      image: ghcr.io/emrggul/mini-rpki:v0.9
      env:
        # Interface between router and MiniRPKI.
        MINIRPKI_ADDRESS: 172.31.1.2/30
        MINIRPKI_PORT: "8282"
        # Optional: RTR v0 is the default. Uncomment for RTR v1.
        # MINIRPKI_RTR_VERSION: "1"
      binds:
        # Keeps VRPs when you destroy and redeploy the topology.
        - ./minirpki-data:/data

  links:
    - endpoints: ["mini-rpki:eth1", "YOUR-ROUTER:ethN"]
```

Containerlab creates `eth1` for that link. MiniRPKI assigns
`MINIRPKI_ADDRESS` to it and listens on `MINIRPKI_PORT`. The Containerlab
management address is separate and is not the router's RTR endpoint.

RTR v0 (RFC 6810) is the default. To use RTR v1 (RFC 8210), uncomment
`MINIRPKI_RTR_VERSION: "1"` in the `env:` block above and redeploy.

## 4. Configure the router

Configure the other address on the point-to-point link and point RTR at
MiniRPKI. This is an FRRouting example:

```text
interface ethN
 ip address 172.31.1.1/30
!
rpki
 rpki cache tcp 172.31.1.2 8282 source 172.31.1.1 preference 1
!
```

For another router platform, use its RTR configuration syntax. The important
values are MiniRPKI's data-link IP and port, plus the router source IP where
the platform supports configuring it.

### Nokia SR OS IPv4 example

Assuming the interface address and routing already exist, this configures the
same MiniRPKI address used above:

```text
# Configure the RPKI-Router (RPKI-RTR) cache server session
/configure router "Base" origin-validation rpki-session 172.31.1.2 local-address 172.31.1.1
/configure router "Base" origin-validation rpki-session 172.31.1.2 port 8282
/configure router "Base" origin-validation rpki-session 172.31.1.2 admin-state enable
```

RTR v0 worked with Nokia in the lab, and RTR v1 worked with the FRR lab.

## 5. Deploy and attach a shell

```sh
containerlab deploy -t your-topology.clab.yml
```

In the Containerlab UI, select `mini-rpki` and choose **Attach Shell**. From a
terminal, you can open the same shell with Docker. For a topology named
`rpki-lab` and a node named `mini-rpki`:

```sh
docker exec -it clab-rpki-lab-mini-rpki sh
```

Inside the shell, manage MiniRPKI directly:

```sh
mini-rpki status
mini-rpki interface
mini-rpki list
mini-rpki add 192.0.2.0/24 --asn 65001
mini-rpki add 2001:db8:200::/48 --asn 65001
mini-rpki remove 192.0.2.0/24 --asn 65001
exit
```

This is the normal management workflow. MiniRPKI has no network management
port. Its commands use a local Unix socket inside the container.

To authorize more-specific routes, set a maximum length:

```sh
mini-rpki add 192.0.2.0/24 --asn 65001 --maxlen 25
```

Removal must match the exact prefix, ASN, and maximum length. Run
`mini-rpki --help` inside the shell for the full command list.

On its first boot, MiniRPKI includes two demonstration VRPs:

```text
192.0.2.0/24          maxlen 24  AS65001
2001:db8:100::/48     maxlen 48  AS65001
```

Run `mini-rpki clear` inside the shell to remove every VRP before creating
your own dataset. This change is persistent when you use the `/data` bind
mount from the topology example.

### Provide an initial dataset as JSON

To start with your own records instead of the demonstration records, create
`minirpki-data/vrps.json` before the first deployment. MiniRPKI loads that
file at startup. After deployment, manage records through the MiniRPKI shell
with `add`, `remove`, and `clear`; host-side JSON editing is not the supported
update workflow.

```json
[
  {
    "prefix": "192.0.2.0/24",
    "maxLength": 24,
    "asn": 65001
  },
  {
    "prefix": "2001:db8:200::/48",
    "maxLength": 48,
    "asn": 65002
  }
]
```

## 6. Verify the router

On FRR, verify the session, received records, and actual BGP result:

```text
show rpki cache-connection
show rpki prefix-table
show bgp ipv4 unicast 192.0.2.0/24
```

Do not treat a TCP connection alone as success. The detailed BGP output should
show `rpki validation-state`; the router calculates that state from the VRPs
served by MiniRPKI.

On Nokia SR OS, check the session and received database with:

```text
show router origin-validation rpki-session
show router origin-validation database
```

## Mutual TLS with Nokia SR OS

Complete the plaintext quick start first. This section changes the
MiniRPKI-to-router RTR connection to mutual TLS; it is separate from the
ordinary router configuration above.

> **TLS laboratory scope:** Mutual TLS is provided for laboratory
> interoperability and concept validation. MiniRPKI does not claim full RFC
> 8210 Section 9.2 TLS compliance or production-grade PKI behavior. It does
> not perform CRL/OCSP revocation checking, does not enforce DNS-SAN or
> wildcard policy for operator-provided server certificates, and relies on
> router configuration to validate the cache server DNS SAN. Use only in a
> trusted lab.

Set these values in the MiniRPKI node's `env:` block, then deploy the lab:

```yaml
# Nokia SR OS uses RTR v0. Use "1" for an RTR v1 client.
MINIRPKI_RTR_VERSION: "0"
MINIRPKI_TLS_BOOTSTRAP: "true"
MINIRPKI_TLS_LISTEN: "172.31.1.2:324"
MINIRPKI_TLS_SERVER_DNS: "minirpki.lab"
MINIRPKI_TLS_ROUTER_IP: "172.31.1.1"
MINIRPKI_TLS_DIR: "/data/tls-rsa"
```

`MINIRPKI_TLS_BOOTSTRAP: "true"` selects TLS mode. In this mode MiniRPKI
opens only the TLS RTR listener on port `324`; plaintext port `8282` is not
available. On its first start, MiniRPKI creates a lab CA, cache certificate,
and router certificate/key in `minirpki-data/tls-rsa`. Later starts preserve a
complete credential set. The generated certificates use RSA with Digital
Signature and Key Encipherment usage for SR OS compatibility.

### Copy generated credentials to CF3

Run these commands on the Containerlab host after MiniRPKI has started.
Replace `<lab-name>` and `<node-name>` with your Containerlab lab and Nokia
node names. `/home/sros/flash3` is the SR OS container path for `cf3:`; it
does not change with the node name.

```sh
docker cp minirpki-data/tls-rsa/ca.pem \
  clab-<lab-name>-<node-name>:/home/sros/flash3/ca.pem

docker cp minirpki-data/tls-rsa/router.pem \
  clab-<lab-name>-<node-name>:/home/sros/flash3/router.pem

docker cp minirpki-data/tls-rsa/router-key.pem \
  clab-<lab-name>-<node-name>:/home/sros/flash3/router-key.pem
```

Inside SR OS, these files appear as `cf3:\ca.pem`, `cf3:\router.pem`, and
`cf3:\router-key.pem`. Do not copy `server-key.pem`; it is MiniRPKI's private
key and stays in `minirpki-data/tls-rsa`.

### Nokia TLS RTR session

First create a Nokia TLS client profile named `rpki-tls-client-profile`. It
must use `cf3:\router.pem` and `cf3:\router-key.pem` as the router identity,
and trust `cf3:\ca.pem` for the MiniRPKI server certificate. Detailed Nokia
TLS, certificate-profile, and trust-anchor configuration is intentionally
excluded from this guide because it varies by SR OS release and local PKI
policy.

Need a Nokia SR OS TLS reference configuration? Open a GitHub Issue with your
SR OS release, Containerlab topology details, and relevant non-sensitive log
output. Do not publish private keys, certificates, passwords, or production
configuration.

Then configure the RPKI session for the TLS port and bind that profile:

```text
/configure router "Base" origin-validation rpki-session 172.31.1.2 local-address 172.31.1.1
/configure router "Base" origin-validation rpki-session 172.31.1.2 port 324
/configure router "Base" origin-validation rpki-session 172.31.1.2 tls-transport-security tls-client-profile "rpki-tls-client-profile"
/configure router "Base" origin-validation rpki-session 172.31.1.2 tls-transport-security server-san-dns-auth "minirpki.lab"
/configure router "Base" origin-validation rpki-session 172.31.1.2 admin-state enable
```

The server DNS name must match `MINIRPKI_TLS_SERVER_DNS`; it authenticates the
server certificate SAN and is not DNS resolution. Verify the TLS session with:

```text
show router origin-validation rpki-session
show router origin-validation database
```

### Use certificates you already have

Keep `MINIRPKI_TLS_BOOTSTRAP: "true"`; it selects TLS mode whether the
credentials are generated or supplied by you. Put your existing files in the
selected TLS directory with these fixed names:

```text
minirpki-data/tls-rsa/ca.pem
minirpki-data/tls-rsa/server.pem
minirpki-data/tls-rsa/server-key.pem
```

`ca.pem` must contain the CA that issued router client certificates. On boot,
MiniRPKI uses this complete three-file set as-is. It creates lab credentials
only when all three files are absent. A partial set is rejected so it cannot
silently mix your PKI with generated files. The router certificate must have
an IP SAN matching the router's actual RTR source address.
