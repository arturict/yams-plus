---
title: Install
description: Bootstrap a pinned CLI, then let the wizard do the talking.
---

## 1. Install the signed release

Each release is built by this repository's GitHub Actions workflow, and its
checksum file is signed with Sigstore keyless signing. The bootstrapper
verifies that signature with `cosign`, so
[install cosign](https://docs.sigstore.dev/cosign/system_config/installation/)
first. Then fetch the bootstrapper from the release tag, read it, and run it:

```sh
curl -fLO https://raw.githubusercontent.com/arturict/yams-plus/v0.1.0/install.sh
less install.sh
sudo sh install.sh --version 0.1.0
```

It installs Docker from Docker's official repository only when Docker is
missing, downloads the archive, the checksum file and its Sigstore bundle for
that version, refuses to continue unless the bundle was signed by this
repository's release workflow for a `v*` tag, checks the archive against the
signed checksum, places `yamsplus` in `/usr/local/bin`, then runs
`yamsplus install`. It does not clone a moving branch or execute a second
remote script.

To build from source instead, on a supported host:

```sh
git clone https://github.com/arturict/yams-plus.git
cd yams-plus
go test ./...
go build -trimpath -o yamsplus ./cmd/yamsplus
sudo ./yamsplus install
```

The host is checked before the first question, so a missing Docker, an
unsupported distribution or a missing `sudo` stops the install before you have
typed anything. When the host is ready, the wizard begins:

```text
YAMS Plus beta — fewer dashboards, more movie night.
Nothing is published and you will add indexers yourself in Prowlarr.
Admin username [admin]:
```

After the last answer, every check is listed with its result before anything
is written, for example:

```text
healthy          architecture         linux/amd64
healthy          docker               29.8.1
healthy          compose              5.5.1
```

:::caution[Signature checks]
Do not run the bootstrapper with `--skip-signature` for a real installation.
That flag exists for controlled local artifact testing, not for turning “trust
me, bro” into a verification strategy.
:::

## 2. Follow the wizard

Questions show their default in brackets. Press Enter to accept it. Secrets are hidden and written as root-readable files under `/etc/yamsplus/secrets/`.

The first image pulls can take several minutes. On success you will see:

```text
Configuring Jellyfin through its API...
Securing and connecting the Arr applications...
Syncing the selected Recyclarr/TRaSH profiles...
Action required: add at least one legal indexer in Prowlarr.
```

Continue with [the only manual step](/prowlarr/).

## Rerun safely

```sh
sudo yamsplus plan
sudo yamsplus apply
```

On a prepared or resumed installation, `apply` securely asks for any provider
secret that is still missing. It asks for the shared admin password when a
service still has to set up its login, such as a module you just enabled, and
on every run of a torrent or combined install, because qBittorrent signs in
with that password and YAMS Plus never stores it. Automation may supply it as
`YAMSPLUS_ADMIN_PASSWORD` and the provider secrets through
`YAMSPLUS_SECRET_USENET_USERNAME`, `YAMSPLUS_SECRET_USENET_PASSWORD`,
`YAMSPLUS_SECRET_SUBTITLES_USERNAME` and
`YAMSPLUS_SECRET_SUBTITLES_PASSWORD`. Do not put those values in the YAML file
or shell history.

After a converged installation, `plan` reports every generated file as `unchanged`, while `apply` verifies live settings without duplicating clients, applications, libraries or profiles.
