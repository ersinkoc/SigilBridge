# Debian Packaging Skeleton

This directory contains a minimal Debian packaging path for tagged SigilBridge release artifacts.

Build flow:

```bash
packaging/deb/build.sh v1.0.0 linux amd64
```

The script expects a release tarball named like:

```text
sigilbridge_v1.0.0_linux_amd64.tar.gz
```

It creates a package layout under `packaging/deb/work` and writes a `.deb` with `dpkg-deb` when available.
