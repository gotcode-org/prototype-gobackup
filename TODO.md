# GoBackup v2.0 Roadmap

## Phase 1: Archive Introspection & Display Polish
- [ ] **Simplify Backup List**: Remove the redundant `Modified Time` column from `gbctl backup list`, as the precise timestamp is already permanently encoded in the archive filename (e.g. `_20260925_124019`).
- [ ] **Implement Archive Deep-Dive Command**: Create `gbctl backup info <archive_name>` to provide a detailed, single-archive dashboard.
  - Traverse the filesystem to map the selected archive to its chronological chain.
  - Display the root `[FULL]` anchor and every associated `[INC]` diff attached to it.
  - Display total chain disk footprint, storage tier location (Hot/Cold), and physical paths.
