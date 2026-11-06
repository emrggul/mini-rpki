# Containerlab lab

The active deployment uses [Containerlab](containerlab.md). Work from the
repository root in a Linux terminal.

Enter MiniRPKI:

Use the Containerlab UI's **Attach Shell** action for the `mini-rpki` node.

Create one record with one line:

```sh
mini-rpki add 192.0.2.0/24 --asn 65001
mini-rpki list
```

Enter the FRR router with `./lab/containerlab/lab router`.

Then run `show rpki prefix-table` and `show bgp ipv4 unicast`.
The [containerlab guide](containerlab.md) covers commands, addresses, timing,
manual record changes, persistence, and lifecycle. No scenario selection is needed.
