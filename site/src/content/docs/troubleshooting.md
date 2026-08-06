---
title: Troubleshooting
description: Start with evidence, not twelve random restarts.
---

Run:

```sh
sudo yamsplus doctor
sudo yamsplus logs SERVICE
sudo yamsplus plan
```

## `action-required`

This usually means Prowlarr has no indexer. Add and test one, then run doctor again.

## A container is unhealthy

Check its log and the pinned version in `/opt/yamsplus/stack.lock.yaml`. Do not switch the image to `latest`; that trades one known problem for a surprise party.

## Imports fail

All download clients and Arr apps see the same `/data` tree. Confirm files remain under `/srv/yamsplus/downloads` and media under `/srv/yamsplus/library`. Avoid remote-path mappings unless the downloader genuinely runs elsewhere.

## Torrent traffic stops

When VPN is enabled, stopped or unhealthy Gluetun intentionally removes qBittorrent egress. Restore the VPN first. This is the kill-switch working, not qBittorrent being dramatic.

## Plugin audit fails

```sh
sudo yamsplus plugins audit --json
sudo yamsplus restart
sudo yamsplus plugins audit
```

YAMS Plus will not force-install a plugin whose manifest lacks a Jellyfin 10.11-compatible build. A missing required plugin blocks the beta release visibly.
