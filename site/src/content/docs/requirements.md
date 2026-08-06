---
title: Requirements
description: Know what the installer expects before it starts moving furniture.
---

YAMS Plus beta supports a fresh **Debian 13** or **Ubuntu 24.04** amd64 server with systemd. Docker Engine and the Compose plugin may already be installed; the bootstrapper can install them from Docker's official repository when needed.

## Recommended host

- 4 CPU cores and 8 GB RAM.
- An SSD for application data and enough media storage for your own use.
- A user with `sudo` access.
- Working DNS, NTP and outbound HTTPS.
- A private LAN address and/or a connected Tailscale address.

Hardware transcoding is detected during preflight. `/dev/dri` is mounted only when you choose it and the device exists.

## Bring these with you

- One administrator username and a strong password. It bootstraps every selected local UI; YAMS Plus does not retain the shared password afterward.
- For Usenet: an NNTP hostname, username and password. Port 563 or 443 with certificate verification is mandatory.
- For torrent VPN: a Proton or generic WireGuard/OpenVPN configuration.
- For subtitles: provider credentials if your selected provider requires them.
- At least one legal Prowlarr indexer that you add yourself.

:::caution[Media rights still apply]
Only request or download media you are authorised to obtain. YAMS Plus does not include indexers, media, shared accounts or a magic legal invisibility cloak.
:::

## Network rule

The wizard accepts loopback, RFC1918 LAN and Tailscale IPv4 addresses. It rejects public bind addresses and `0.0.0.0`. Nothing is exposed to the public internet by default.
