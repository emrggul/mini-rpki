# WSL lab

The active deployment uses [containerlab](containerlab.md). Open Ubuntu with
`wsl -d Ubuntu-22.04`, then work in `~/src/MiniRPKI`.

Enter MiniRPKI:

```sh
docker exec -it clab-minirpki-mini-rpki sh
```

Create one record with one line:

```sh
mini-rpki add 192.0.2.0/24 --asn 65001
mini-rpki list
```

Enter the FRR router from a separate WSL terminal:

```sh
docker exec -it clab-minirpki-router vtysh
```

Then run `show rpki prefix-table` and `show bgp ipv4 unicast`.
The [containerlab guide](containerlab.md) covers commands, addresses, timing,
manual record changes, persistence, and lifecycle. No scenario selection is needed.
