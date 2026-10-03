# GoBackup v2.1 Roadmap

## Phase 3: Enhanced Notifications & Reporting
- [ ] **Storage Telemetry Injection**: Embed real-time storage tier statistics into the post-batch notifications (Discord webhooks, SMTP Email, etc).
  - Refactor the disk traversal and math logic from the `GetStatus` RPC into a shared daemon utility.
  - Append a Markdown-formatted table containing Tier, Path, Archive Count, Used Space, and Free Space directly into the `desc` payload of the final Batch Digest.
- [ ] **Live Progress Bars & ETA**: Implement a file-count based progress tracker for running backups.
  - Run a pre-flight \`find /paths | wc -l` over SSH to get the total file count.
  - Enable verbose mode (`tar -cvzf`) so the remote server streams processed filenames to `stderr`.
  - Intercept the `stderr` stream in the daemon, count the lines as files complete, and broadcast a real-time `(processed/total) %` to the TUI.
