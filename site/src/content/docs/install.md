---
title: Install
description: Bootstrap a pinned CLI, then let the wizard do the talking.
---

## 1. Choose the beta path

There is no signed public CLI release yet. The current source is public for
inspection and testing. On a supported host, clone it, run the tests and build
the CLI locally:

```sh
git clone https://github.com/arturict/yams-plus.git
cd yams-plus
go test ./...
go build -trimpath -o yamsplus ./cmd/yamsplus
sudo ./yamsplus install
```

Expected beginning:

```text
YAMS Plus preflight
healthy          operating-system     Debian 13
healthy          architecture         amd64
```

The checked-in `install.sh` is reserved for signed release bundles. It installs
Docker only when it is missing, verifies the CLI checksum and Sigstore identity,
places `yamsplus` in `/usr/local/bin`, then runs `yamsplus install`. It does not
clone a moving branch or execute a second remote script.

:::caution[Beta source installs]
Until a signed public beta release exists, build locally with `go build`. Do not
run the bootstrapper with `--skip-signature` for a real installation. That flag
exists for controlled local artifact testing, not for turning “trust me, bro”
into a verification strategy.
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
