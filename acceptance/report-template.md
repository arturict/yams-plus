# YAMS Plus beta acceptance report

- Date and operator:
- Host OS and hardware (redacted):
- YAMS Plus version/commit:
- Active-input time:
- Time to indexer checkpoint:

## Locked components

Record every service version and image digest from `stack.lock.yaml`.

## Wizard selection

Record modules, downloader mode, quality profiles, subtitle languages and bind
classes. Never paste secret values or full private addresses.

## Automated gates

- [ ] Go unit and race tests
- [ ] Contract tests
- [ ] Debian 13 VM scenario
- [ ] Ubuntu 24.04 VM scenario
- [ ] Second apply reports zero changes
- [ ] Secret, filesystem and container scans
- [ ] SBOM and checksums produced
- [ ] Website build, links, accessibility and screenshots

## Authorised media path

Record only titles the owner explicitly authorised for this test.

- [ ] Film requested in Seerr
- [ ] Small series requested by a normal user in Jellyfin Enhanced
- [ ] Automatic approval visible in Seerr
- [ ] Prowlarr → downloader → Arr → Jellyfin path proven
- [ ] Naming, paths, metadata, subtitles and playback proven

## Plugin matrix

- [ ] Enhanced and File Transformation injection / Seerr search
- [ ] Plugin Pages and Custom Tabs navigation without a duplicate
- [ ] Editor's Choice shelf
- [ ] Intro Skipper two-episode fixture and skip marker
- [ ] Playback Reporting and Reports entry after playback
- [ ] Metadata-provider searches
- [ ] Every required plugin active after reboot

## Recovery

- [ ] Encrypted backup created
- [ ] Restore tested in an isolated root
- [ ] Host rebooted
- [ ] Reapply is idempotent
- [ ] Doctor, browser smoke test and secret scan green

## Known limits and rollback

Document failures, deferred work and the exact non-destructive rollback steps.
