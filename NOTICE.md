# Third-party notices

YAMS Plus is an independent project inspired by the scope and onboarding
experience of [YAMS](https://yams.media/). It is not affiliated with or
endorsed by YAMS. No YAMS website copy, screenshots, branding, or source code
are included.

The generated stack integrates independent open-source projects. Their names
and trademarks belong to their respective owners. Container images and plugin
packages retain their upstream licenses.

## Projects the generated stack installs

Pinned by version and digest in `stack/stack.lock.yaml`:

| Project | Upstream |
| --- | --- |
| Jellyfin | https://jellyfin.org/ |
| Seerr | https://github.com/seerr-team/seerr |
| Radarr | https://radarr.video/ |
| Sonarr | https://sonarr.tv/ |
| Prowlarr | https://prowlarr.com/ |
| Bazarr | https://www.bazarr.media/ |
| SABnzbd | https://sabnzbd.org/ |
| qBittorrent | https://www.qbittorrent.org/ |
| Gluetun | https://github.com/qdm12/gluetun |
| Recyclarr | https://recyclarr.dev/ |
| Audiobookshelf | https://www.audiobookshelf.org/ |
| Shelfmark | https://github.com/calibrain/shelfmark |

The Radarr, Sonarr, Prowlarr, Bazarr, SABnzbd and qBittorrent images are the
[LinuxServer.io](https://www.linuxserver.io/) builds.

## Quality profiles

`stack/recyclarr/radarr.yaml.tmpl` and `stack/recyclarr/sonarr.yaml.tmpl`
reference quality profile and custom format identifiers published by
[TRaSH Guides](https://trash-guides.info/)
([TRaSH-Guides/Guides](https://github.com/TRaSH-Guides/Guides), MIT licence).
Only the identifiers are referenced; Recyclarr fetches the definitions
themselves from TRaSH Guides at sync time.

## Jellyfin plugin repositories

Installing YAMS Plus adds these third-party plugin manifests to Jellyfin as
trusted sources, in addition to the official repository. They are listed here
so the supply chain is visible before you install:

- Jellyfin Enhanced — https://raw.githubusercontent.com/n00bcodr/jellyfin-plugins/main/10.11/manifest.json
- IAmParadox27 Plugins — https://www.iamparadox.dev/jellyfin/plugins/manifest.json
- Intro Skipper — https://intro-skipper.org/manifest.json
- Editor's Choice — https://raw.githubusercontent.com/lachlandcp/jellyfin-editors-choice-plugin/main/manifest.json

## Go dependencies

The `yamsplus` binary links, per `go.mod`:

- filippo.io/age and filippo.io/hpke (BSD-3-Clause)
- golang.org/x/crypto, golang.org/x/term and golang.org/x/sys (BSD-3-Clause)
- gopkg.in/yaml.v3 (MIT and Apache-2.0)

Their licence texts travel with the module cache and must accompany any
binary distribution.
