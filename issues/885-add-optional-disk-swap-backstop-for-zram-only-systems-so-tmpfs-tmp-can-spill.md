# 885 — Add optional disk swap backstop for zram-only systems so tmpfs /tmp can spill

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Enhancement
**Related**: loom session on 2026-10-23 (go test temp files filled `/tmp`)

---

## 1. Problem & Motivation
On Fedora (systemd `tmp.mount`), `/tmp` is a tmpfs, so its files live in RAM. The only swap on the
test machine is zram, which is compressed RAM, so there is no disk to spill onto. Under load,
Go builds failed with "disk quota exceeded" because the per-user tmpfs quota was full, and the whole
system slowed down because the zram swap was also full.

The goal is to keep `/tmp` in RAM (it spares the disk) and still let the kernel spill to disk
when RAM and zram are both full.

## 2. Technical Specification / Findings
- Observed: `/tmp` tmpfs 11.6G with a 9.5G user quota (`usrquota`); `zramctl` shows 8G zram swap,
  7.9G data compressed to 5G; `free -h` showed 287Mi free with swap fully used.
- Linux can swap tmpfs pages, but only to a swap device on disk. zram cannot take that overflow.
- Proposal: add a disk swap file as a low-priority backstop, below zram (e.g. zram priority 100,
  disk swap priority 10). The kernel only uses it when zram is full, so disk writes stay small.
- Open question: whether harnez should only document this, or offer a setup command that creates the
  swap file and sets its priority.

## 3. Implementation & Verification Plan
1. Document the setup steps (swap file size, `mkswap`, `swapon -p 10`, `/etc/fstab` entry).
2. Check `swapon --show` and `zramctl` after setup; confirm zram keeps priority 100.
3. Measure with a memory-heavy test that fills zram; confirm the disk swap is used only then.
4. Decide whether harnez should provide a command for this.
