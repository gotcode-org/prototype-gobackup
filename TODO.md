# GoBackup v2.0 Roadmap

## Phase 1: Archive Introspection & Display Polish
- [ ] **Simplify Backup List**: Remove the redundant `Modified Time` column from `gbctl backup list`, as the precise timestamp is already permanently encoded in the archive filename (e.g. `_20260925_124019`).
- [ ] **Implement Archive Deep-Dive Command**: Create `gbctl backup info <archive_name>` to provide a detailed, single-archive dashboard.
  - Traverse the filesystem to map the selected archive to its chronological chain.
  - Display the root `[FULL]` anchor and every associated `[INC]` diff attached to it.
  - Display total chain disk footprint, storage tier location (Hot/Cold), and physical paths.

## Phase 2: Archive Integrity & Auditing
- [ ] **Cryptographic Hash Tracking**: Automatically compute a SHA-256 checksum immediately after a `.tar.gz` archive is finalized.
  - Store the checksum persistently in a GitOps-friendly YAML registry under `/etc/gobackup/conf.d` (so changes can be audited in Git) and cache it in the embedded SQLite DB for lightning-fast reads.
  - Integrate a validation sweep into the `CleanupOldBackups` and `CleanupColdStorage` engines to passively detect bit-rot or unauthorized modifications to archival chains before they are relied upon for restorations.
- [ ] **Blockchain Chain-of-Custody**: Structurally link the hashes of incremental backups to cryptographically guarantee the sequence.
  - When calculating the hash for an `[INC]` backup, append the hash of the preceding `[FULL]` or `[INC]` archive to the payload before hashing.
  - This effectively turns the incremental sequence into a cryptographic Merkle tree (blockchain). If a malicious actor alters a `[FULL]` anchor or an early `[INC]`, the hashes of every single subsequent incremental backup will instantly invalidate, immediately flagging the entire chain as compromised.
