# Changelog

## [2.0.0] - 2026-10-02
### Added
- **Storage Tier Statistics**: Implemented dynamic traversal of active
  global/job-level paths, reporting real-time disk utilization metrics and
  active/cold counts per tier via a new matrix format in 'gbctl status'.
- **Archive Introspection API**: Added 'gbctl backup info <anchor_file>' command
  to instantly map out nested chronological diffs for any given root backup
  anchor.
- **Cryptographic Registry**: Automated inline SHA-256 computation over the raw
  SSH I/O pipe without disk re-reading overhead. Checksums are natively linked
  to GitOps configurations (/etc/gobackup/conf.d/hashes) and mirrored into
  embedded SQLite for zero-latency lookups.
- **Blockchain Chaining Architecture**: 'INC' payloads are cryptographically
  salted with the hash of their parent archive, turning sequences into Merkle
  trees to physically prevent tampering or partial chain restoration.
- **Passive Cryptographic Sweep**: The 'CleanupOldBackups' background daemon
  actively interrogates cold hashes against raw disk bytes every time it cycles,
  locking down the gRPC 'RestoreBackup' pipe upon integrity failure.

### Removed
- Deprecated 'MODIFIED' timestamp display column in 'gbctl backup list',
  delegating sequence tracking exclusively to archive namespace boundaries.

All notable changes to the GoBackup project will be documented in this file.

## [1.0.0] - 2026-09-25

### Added
- **Native GNU Tar Engine**: Complete orchestration of standard `tar` operations
  over SSH, entirely eliminating the need for client-side agents.
- **Smart Incremental Chains**: Full implementation of GNU tar's `--listed-
  incremental` (`-g`) flag. Automatically orchestrates `[FULL]` anchor backups
  and sequences their attached `[INC]` snapshots.
- **Multi-Tier Archiving (Cold Storage)**: Dynamically routes expired local Hot
  Storage backups to a secondary high-capacity NFS/NAS Cold Storage mount,
  enforcing independent retention limits for each tier.
- **Docker Volume Awareness**: Intelligently backups live Docker volumes by
  spawning disposable ephemeral utility containers, seamlessly bypassing root-
  UID permission denied errors.
- **Ephemeral Restorations**: Supports native Docker volume restorations by
  streaming sequential tarball chains backwards over SSH into a sterile
  `debian:stable-slim` container directly connected to the target docker-volume.
- **Unified Multi-Channel Notifications**: Built a robust, asynchronous
  notification dispatcher supporting: - Microsoft 365 Graph API (Native OAuth2
  HTML Email SaaS cards) - SMTP - Discord Webhooks - Generic / Custom Gotcode
  Webhooks
- **Batch Processing & Digests**: Cron executions are debounced and grouped into
  unified "Queue Started" and "Execution Digest" summary markdown tables,
  massively reducing notification spam.
- **Storage Resilience**: Implemented strict 3-second NFS `verifyNFSMount()`
  sentinel checks that proactively abort backups before they can freeze the
  daemon.
- **Live Terminal UI (TUI)**: Integrated a native Bubble Tea TUI that seamlessly
  streams live remote backup logs via `gbctl job run --attach`.
- **GitOps Integrations**: Config updates pushed from `gbctl` automatically
  rewrite local declarative YAML templates.

### Changed
- Refactored entire engine pruning sequence to operate on Chronological Chains
  rather than raw file limits.
- Overhauled `gbctl` status dashboards to dynamically handle dynamic tabs,
  cross-tier scanning, and real-time cron queue views.

### Fixed
- Fixed critical Linux kernel `invalid cross-device link` (`EXDEV`) errors when
  routing Cold Storage backups across physical block partitions by wrapping
  `os.Rename` with a custom streaming `io.Copy`.
- Fixed GNU `tar: unrecognized option` crashes during ephemeral container
  restoration by replacing Busybox `alpine` with GNU `debian`.
- Fixed SSH daemon stall conditions when targeting deeply nested or locked root
  filesystem structures.
