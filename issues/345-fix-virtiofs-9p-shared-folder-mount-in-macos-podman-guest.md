# 345 — Fix virtiofs/9p shared folder mount in macos-podman guest

**Status**: Open
**Priority**: P2
**Severity**: Medium
**Category**: macos-podman / guest integration

---

## Summary

The host directory mounted via `-v` (e.g. `/home/uwe/projects/harnez:/shared`) is not
accessible inside the macOS guest. The `mount` output shows no virtiofs/9p entry and
`/shared` does not exist in the guest filesystem.

## Observed

- Container started with `Shared: /home/uwe/projects/harnez -> /shared` shown in run output
- Inside guest: `ls /shared` → path does not exist
- `mount` inside guest: only APFS volumes and `macOS Base System` visible
- Workaround used: `scp -P 2222` to copy files directly to guest `~/Downloads/`

## Investigation

- Check whether the container's virtiofs/9p socket is exposed to QEMU (`-virtfs` or `-device virtio-9p`)
- Check if macOS guest needs explicit `mount_9p` or kernel extension for the share to appear
- Consider using SPICE/WebDAV or an SMB share as an alternative transport

## Next Steps

- [ ] Inspect `podman inspect macos-kvm` mounts vs QEMU launch args for virtiofs config
- [ ] Test manual `mount_9p` or `mount -t virtiofs` inside guest
- [ ] If virtiofs unsupported on macOS guest, implement SMB or scp-based sync helper in `macos-podman`
