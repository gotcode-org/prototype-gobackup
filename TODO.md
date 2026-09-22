# Phase 3: Incremental Backups & Restore Subsystem

## 1. Incremental State Management
* **JobConfig Options**: Add `Incremental: true` and `FullBackupInterval: 7d` fields to the `JobConfig` struct.
* **Remote Metadata**: The native GNU tar `--listed-incremental` (`-g`) flag requires a snapshot file to track inodes and timestamps. We will store this state safely in the limited user's home directory (e.g., `/home/backup/.gobackup/snapshots/<job_name>.snar`).

## 2. Backup Execution Engine Updates
* When a backup job triggers, the engine will determine if it should be a **Full** or **Incremental** run based on a 7-day cycle.
* **Full Run (Day 1/8/15)**: Connect via SSH, delete the existing `.snar` file to reset the chain, and execute the backup. Save the local tarball as `SERVER_JOB_FULL_TIMESTAMP.tar.gz`.
* **Incremental Run (Days 2-7)**: Connect via SSH, leave the `.snar` file intact so tar updates it, and execute the backup. Save the local tarball as `SERVER_JOB_INC_TIMESTAMP.tar.gz`.
* **Chain-based Retention Pruning**: 
  * GNU tar incrementals cannot be merged. If a Full backup is deleted, all subsequent incrementals are broken.
  * Therefore, `RetentionCount` will be redefined as `RetentionDays` (enforced in 7-day increments: 7, 14, 21, etc.).
  * The pruner will operate on **Chains** rather than individual files. To guarantee a minimum of 7 days of history, the engine must always keep `(RetentionDays / 7) + 1` full chains.
  * *Example (7-Day Retention)*: On Day 8, a new Full backup is taken, starting Chain 2. However, Chain 1 (Days 1-7) CANNOT be deleted yet, otherwise the user would only have 1 day of history. On Day 15, when Chain 3 begins, Chain 1 is finally purged, ensuring the user always has between 7 and 14 days of recoverable history.


## 2.5 Docker Volume Implementation
* When backing up Docker Volumes, the `tar` command executes inside a disposable `alpine` container.
* To persist the `.snar` snapshot file between runs, the engine must bind-mount the host's snapshot directory into the container.
* Execution command format:
  ```bash
  docker run --rm \
    -v "VOLUME_NAME:/data:ro" \
    -v "/home/backup/.gobackup/snapshots:/snapshots" \
    -u "$(id -u):$(id -g)" \
    alpine tar -cvzf - -g /snapshots/VOLUME_NAME.snar /data
  ```
* The `-u` flag is critical to ensure the `.snar` file is written with the `backup` user's permissions, rather than `root`.

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
