# GoBackup v2.1 Roadmap

## Phase 3: Enhanced Notifications & Reporting
- [x] **Storage Telemetry Injection**: Embed real-time storage tier statistics into the post-batch notifications (Discord webhooks, SMTP Email, etc).
  - Refactor the disk traversal and math logic from the `GetStatus` RPC into a shared daemon utility.
  - Append a Markdown-formatted table containing Tier, Path, Archive Count, Used Space, and Free Space directly into the `desc` payload of the final Batch Digest.
- [ ] **Live Progress Bars & ETA**: Implement a file-count based progress tracker for running backups.
  - Run a pre-flight \`find /paths | wc -l` over SSH to get the total file count.
  - Enable verbose mode (`tar -cvzf`) so the remote server streams processed filenames to `stderr`.
  - Intercept the `stderr` stream in the daemon, count the lines as files complete, and broadcast a real-time `(processed/total) %` to both the TUI and the `gbctl status` active job response.
- [ ] **Fluid Batch Progress Tracker**: Implement a secondary global progress bar for active cron batches.
  - Track `BatchTotal` and `BatchCompleted` integers in the global queue state.
  - Calculate overall progress dynamically: `((Completed * 100) + ActiveJob%) / Total`.
  - Broadcast the output to the TUI and `gbctl status` formatted as: `Batch Progress: 62.5% Complete (2/4 Jobs Finished)`.
- [ ] **Live CLI Dashboard**: Replace the complex TUI log streaming with a simple `gbctl status --watch` flag.
  - Implement a 1-second ticker loop in the status command.
  - Clear the terminal screen on each tick and fetch the latest `GetStatus` RPC payload.
  - Deprecate and remove the `gbctl daemon attach` command entirely.
