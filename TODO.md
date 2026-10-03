# GoBackup v2.1 Roadmap

## Phase 3: Enhanced Notifications & Reporting
- [ ] **Storage Telemetry Injection**: Embed real-time storage tier statistics into the post-batch notifications (Discord webhooks, SMTP Email, etc).
  - Refactor the disk traversal and math logic from the `GetStatus` RPC into a shared daemon utility.
  - Append a Markdown-formatted table containing Tier, Path, Archive Count, Used Space, and Free Space directly into the `desc` payload of the final Batch Digest.
