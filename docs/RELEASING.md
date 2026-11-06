# Releasing MiniRPKI

This repository is not published until its owner creates the GitHub repository
and pushes the source. Do not substitute an imagined GitHub URL in user
documentation before then.

## Before creating a release

1. Review `README.md`, `GETTING_STARTED.md`, `SECURITY.md`, and this file.
2. Run the Go checks below and build the application image.
3. Use a disposable Containerlab lab to verify FRR's explicit `rpki
   validation-state` output for at least Valid, Invalid, and Not Found.
4. Confirm `lab/containerlab/data/` is not being committed and contains no
   user VRPs.
5. Review the `Limitations, security, and troubleshooting` section of the
   README. Do not represent the project as production-ready or as a full RPKI
   validator.
6. Enable GitHub private vulnerability reporting before public announcement.
7. Confirm GitHub Actions has permission to write packages.

## Publish

Create and push an annotated tag such as `v0.1.0`:

```sh
git tag -a v0.1.0 -m 'MiniRPKI v0.1.0'
git push origin v0.1.0
```

The release workflow publishes one universal image:

```text
ghcr.io/OWNER/mini-rpki:v0.1.0
```

It runs normally in Docker. When a Containerlab topology supplies
`MINIRPKI_ADDRESS` and `MINIRPKI_PORT`, the same image configures its `eth1`
data link before starting MiniRPKI. See `examples/containerlab/README.md` for
the exact node/link pattern.

After the workflow completes, make the package public in GitHub Packages if
you want users to pull it without GitHub authentication. Test a clean pull and
deploy before announcing the release.
