# YAMS Plus

[![CI](https://github.com/arturict/yams-plus/actions/workflows/ci.yml/badge.svg)](https://github.com/arturict/yams-plus/actions/workflows/ci.yml)
[![Container security](https://github.com/arturict/yams-plus/actions/workflows/container-security.yml/badge.svg)](https://github.com/arturict/yams-plus/actions/workflows/container-security.yml)
[![Guide](https://img.shields.io/badge/guide-live-7c3aed)](https://yamsplus-guide.vercel.app/)
[![License: GPL-3.0](https://img.shields.io/badge/license-GPL--3.0-blue.svg)](LICENSE)

An opinionated, modular Jellyfin stack that configures the boring bits for you.

YAMS Plus installs Jellyfin, Seerr and Prowlarr, adds the movie, series,
subtitle, downloader and books modules you choose, then connects the apps
through their supported APIs. The one intentional manual stop is adding at
least one indexer you are allowed to use in Prowlarr.

> Independent project inspired by [YAMS](https://yams.media/). Not affiliated
> with or endorsed by YAMS.

## Beta status

This repository is public so the implementation can be inspected, tested and
improved in the open. It is an early beta, not a signed production release.
Configuration convergence, an idempotent second apply, encrypted backup and
restore, restart survival and a credential-free Ubuntu install have passed; the
real authorised media-download flow and several UI-level plugin checks are
still outstanding. The current evidence and known limits are recorded in the
[beta report](acceptance/local-beta-report.md), including a container scan that
is not green.

The installed stack has no telemetry. The separately deployed documentation
uses privacy-conscious, self-hosted Umami analytics as described in the
[guide privacy page](https://yamsplus-guide.vercel.app/privacy/).

## What it automates

- Jellyfin, Seerr and Prowlarr as the always-on core.
- Optional Radarr, Sonarr, Bazarr, SABnzbd, qBittorrent with Gluetun, Shelfmark
  and Audiobookshelf modules.
- App authentication, root folders, download clients, naming, libraries and
  private LAN, loopback or Tailscale port bindings.
- Recyclarr-managed TRaSH profiles for the selected 1080p and 4K shapes, with
  720p available as a fallback tier the 1080p profile upgrades away from.
- A compatible Jellyfin plugin pack with repeatable installation and auditing.
- Stable `plan`, `doctor --json`, encrypted backup and idempotent `apply`
  workflows.

YAMS Plus does not choose indexers, guess credentials or expose services to the
public internet. It does not authorise downloading media you do not have the
right to obtain.

## Supported beta hosts

- Debian 13 or Ubuntu 24.04
- amd64
- systemd
- Docker Engine with the Compose plugin

ARM64 and other distributions are not supported by this beta.

## Install

Install [cosign](https://docs.sigstore.dev/cosign/system_config/installation/),
then fetch the bootstrapper from the release tag, read it, and run it:

```sh
curl -fLO https://raw.githubusercontent.com/arturict/yams-plus/v0.1.0/install.sh
less install.sh
sudo sh install.sh --version 0.1.0
```

It verifies the Sigstore signature and checksum of the release before
installing `yamsplus` and starting the wizard. To build from source instead,
inspect it first and build it on a supported host:

```sh
git clone https://github.com/arturict/yams-plus.git
cd yams-plus
go test ./...
go build -trimpath -o yamsplus ./cmd/yamsplus
sudo ./yamsplus install
```

The wizard shows defaults before writing anything. For a file-only preview:

```sh
go run ./cmd/yamsplus --root "$(mktemp -d)" install \
  --config examples/usenet.yaml --dry-run
```

Read the complete [requirements](https://yamsplus-guide.vercel.app/requirements/)
and [installation guide](https://yamsplus-guide.vercel.app/install/) before
using it on a real host.

## Development

```sh
go test ./...
go vet ./...
cd site
npm ci
npm run build
npm run test:e2e
```

Generated Compose shapes can be checked with `make compose-check` on a host
with Docker. See [CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull
request and [SECURITY.md](SECURITY.md) for private vulnerability reports.

## License and credits

YAMS Plus is licensed under [GPL-3.0](LICENSE). Third-party project and
trademark notices are in [NOTICE.md](NOTICE.md).
