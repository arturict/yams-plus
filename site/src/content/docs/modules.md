---
title: Optional modules
description: What each switch adds, including the honest caveats.
---

## Movies and series

Radarr and Sonarr receive root folders, download clients, naming, media management, authentication, Prowlarr Full Sync and Recyclarr profiles. Seerr uses the selected default profile and automatically approves requests from imported, non-blocked Jellyfin users without granting admin rights.

## Subtitles

Bazarr receives Arr connections, enabled languages, Forced/HI preferences and provider credentials. The beta acceptance scenario uses German and English.

## Torrent + VPN

qBittorrent runs through Gluetun with a kill-switch. Proton and generic WireGuard/OpenVPN configurations are supported. Port forwarding is optional where the provider supports it.

## Books beta

Shelfmark searches book-capable Prowlarr indexers and hands downloads to SABnzbd or qBittorrent. It writes completed eBooks and audiobooks into folders watched by Audiobookshelf.

Shelfmark is **not** a series monitor. Audiobookshelf offers only basic eBook support. That combination is useful, but it is not Readarr wearing a fake moustache.

## Deliberately absent

Plex, Emby, Lidarr, Portainer, Watchtower and SSO-Auth are not installed. The SSO plugin is archived and described upstream as alpha; beta reliability wins this round.
