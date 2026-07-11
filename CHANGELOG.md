# Changelog

## Unreleased

### Security

- Bind new installations to `127.0.0.1:8080` and generate a one-time random admin password instead of using a documented default credential.
- Replace the guest HTML/noVNC reverse proxy with a trusted bundled noVNC client and an authenticated raw RFB tunnel to guest loopback.
- Generate a random VNC password per desktop VM and stop exposing VNC or websockify ports on the guest network.
- Stage cloud-init data in exclusive random files and verify release binary checksums before installation.
- Upgrade vulnerable frontend dependencies; the committed lockfile reports zero known npm vulnerabilities.

### Fixed

- Deep-copy config snapshots and serialize mutable config access to remove concurrent slice, map, token, and LLM configuration races.
- Return older audit events from cursor pagination instead of repeating the newest page.
- Coalesce polling ticks while a refresh is running so slow Multipass commands cannot accumulate.
- Preserve the configured bind host when overriding only the listen port.

### Changed

- Lazy-load operational panels, editors, terminals, and noVNC. The production entry bundle is approximately 35 KB instead of 1.18 MB.
- Run frontend tests and a high-severity dependency audit in CI before producing release binaries.
- Add regression coverage for bootstrap configuration, cloud-init security invariants, VNC connection metadata, temp files, config cloning, audit pagination, and serialized polling.

### Upgrade Notes

- Existing configuration files keep their current `listen` value and password. Only newly generated configurations receive the safer defaults.
- Existing desktop VMs created with the former websockify template must be reprovisioned with `ubuntu-desktop-novnc.yml` before the VNC tab can connect.
