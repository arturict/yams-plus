---
title: The wizard
description: Every choice, its default, and what happens next.
---

The wizard is intentionally short. It asks about outcomes, not whether you happen to know the exact JSON shape of a Radarr download-client schema.

```text
Movies? [Y/n] y
Series? [Y/n] y
Subtitles? [Y/n] y
Books and audiobooks? [y/N] n
Download method (usenet/torrent/both): usenet
Enable 4K movies too? [Y/n] y
Default movie quality (1080p/2160p) [1080p]:
```

## Download choices

**Usenet** uses SABnzbd and requires NNTP TLS with certificate verification. A VPN is optional and defaults to no; it does not replace TLS.

**Torrent** uses qBittorrent. VPN defaults to yes. With VPN enabled, qBittorrent shares Gluetun's network namespace and cannot continue when Gluetun stops. Torrent without VPN requires an explicit warning acknowledgement.

**Both** configures Usenet as immediate and torrent as a 60-minute fallback.

The provider suggestions are neutral official links during beta. Newshosting and Proton are presented beside generic compatible alternatives. No affiliate URL, tracking or automatic browser launch is active.

## Quality rules

- Movies: 1080p enabled by default; 720p and 2160p optional. Default is 1080p.
- Series: 1080p enabled by default; 720p optional; 2160p is advanced.
- A title has one managed copy. A 1080p profile may temporarily accept 720p, then upgrade to cutoff.
- Original audio is preferred. Compatible SDR/HDR10 is allowed; Dolby Vision without HDR fallback is rejected.

The final summary appears before files or containers are changed. `yamsplus plan --json` provides the same proposed file state without secret values.
