# Phase 3: Incremental Backups & Restore Subsystem

## 1. Incremental State Management
* **JobConfig Options**: Add `Incremental: true` and `FullBackupInterval: 7d` fields to the `JobConfig` struct.
* **Remote Metadata**: The native GNU tar `--listed-incremental` (`-g`) flag requires a snapshot file to track inodes and timestamps. We will store this state safely in the limited user's home directory (e.g., `/home/backup/.gobackup/snapshots/<job_name>.snar`).

## 2. Backup Execution Engine Updates
* When a backup job triggers, the engine will determine if it should be a **Full** or **Incremental** run.
* **Full Run**: Connect via SSH, delete the existing `.snar` file to reset the chain, and execute the backup. Save the local tarball as `SERVER_JOB_FULL_TIMESTAMP.tar.gz`.
* **Incremental Run**: Connect via SSH, leave the `.snar` file intact so tar updates it, and execute the backup. Save the local tarball as `SERVER_JOB_INC_TIMESTAMP.tar.gz`.
* Update the retention pruner so it doesn't delete a Full backup if newer Incremental backups still depend on it!

## 3. CLI Updates: `gbctl backup list`
* Update the `BackupArchive` gRPC protobuf to parse out `FULL` vs `INC` types based on the filename.
* Enhance the CLI tabular output to visually group incremental backups beneath their parent full backup to easily understand the history chain:
  ```text
  SERVER        JOB             TYPE      ARCHIVE
  dev-docker01  system-paths    [FULL]    dev-docker01_system-paths_FULL_20260920.tar.gz
  dev-docker01  system-paths    └-[INC]   dev-docker01_system-paths_INC_20260921.tar.gz
  dev-docker01  system-paths    └-[INC]   dev-docker01_system-paths_INC_20260922.tar.gz
  ```

## 4. The Restore Subsystem (`gbctl backup restore`)
* **CLI Command**: `gbctl backup restore <archive_name> --dest /home/backup/RESTORE`
* **gRPC Endpoint**: `RestoreBackup` (will stream logs via TUI exactly like the run command).
* **Dependency Chain Resolution**:
  * If you select an `INC` file to restore, the daemon will automatically scan backwards to find the parent `FULL` tarball and compile a list of all required files in chronological order.
* **SSH Reverse Streaming**:
  * The daemon will execute `mkdir -p /home/backup/RESTORE && tar -xvzf - -C /home/backup/RESTORE -g /dev/null` on the remote server as the `backup` user (avoiding sudo permission issues).
  * It will sequentially `io.Copy()` the Full tarball, and then the Incremental tarballs, directly into the remote SSH `stdin` pipe, rebuilding the exact filesystem state at the target directory.
