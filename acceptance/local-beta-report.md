# Beta candidate evidence — 2026-08-05

This report contains local implementation evidence, an isolated x1 smoke test
and the public documentation deployment. It is not the completed authorised
media acceptance and it is not a CLI release approval. Local runtime testing
used Docker Desktop on Windows/amd64. The credential-free smoke test used the
live Ubuntu 24.04/amd64 x1 host; the disposable Debian 13 and Ubuntu 24.04 VM
scenarios remain outstanding.

## Candidate state

- Stack lock: all 12 service versions were compared with their upstream latest
  stable releases and every version-tag digest still matched `stack.lock.yaml`.
- Fresh full-module Usenet render: movies, series, subtitles and Books beta;
  1080p default, movie 4K enabled, English and German subtitles, private
  loopback binding.
- Five generated shapes passed `docker compose config`: Usenet, Torrent with
  WireGuard, Both with OpenVPN, movies-only and series-only.
- A fresh full-module x1 bootstrap converged through supported APIs in an
  isolated `/var/tmp/yamsplus-smoke` root with alternate loopback-only ports.
  It used deliberately invalid placeholder provider values, no indexer and no
  request. A second `apply` completed without admin or provider credentials and
  all five generated files reported `unchanged`.
- `doctor --json`: 22 checks, 0 failed, 1 action required. The only action was
  the intentional manual Prowlarr indexer step.
- `plugins audit --json`: all 13 required compatible plugins active.
- Playback Reporting was verified at `MaxDataAge: 3`, the plugin's native
  month granularity corresponding to the configured 90-day policy.
- Jellyfin Enhanced was verified with Seerr enabled, user auto-import enabled,
  Plugin Pages selected, duplicate Custom Tabs navigation disabled and the
  theme selector enabled.
- Shelfmark now retains a service-specific signed automation session. The
  common admin password was not stored and was not requested on the second
  apply. A fresh Books bootstrap restarts Shelfmark exactly once after enabling
  built-in authentication, then accepts only a session that Shelfmark itself
  verifies as an authenticated administrator.
- Prepared installs now securely prompt for missing provider secrets during
  `apply`; empty secret files are rejected. Environment-backed automation is
  available without placing credentials in YAML or shell arguments.
- Linux app, download and media directory ownership and modes are explicitly
  converged to the selected PUID/PGID. This includes intermediate data roots,
  avoiding host-umask failures that only appear on a real Linux filesystem.
- The x1 run exposed and fixed three real-host-only defects: Jellyfin's startup
  connection reset during its first database migration, Recyclarr's unwritable
  `/config/state`, and Shelfmark's delayed transition from no-auth to built-in
  authentication.

## Automated gates

- PASS: `go test ./...`, `go vet ./...`, Staticcheck v0.7.0.
- PASS: govulncheck v1.6.0 with Go 1.26.5, zero reachable vulnerabilities.
- PASS: npm audit, Astro check/build and 10 Playwright accessibility/smoke tests.
- PASS: Gitleaks with runtime test data excluded by the checked-in policy.
- PASS: Trivy 0.73.0 repository scan at High/Critical after upgrading
  `golang.org/x/crypto` to v0.52.0.
- PASS: snapshot archive, SHA-256 checksums and SPDX 2.3 SBOM generation.
  Two clean builds produced the identical archive SHA-256
  `5fe0a278a2da02aa30320cbfd436b74d0cd7060c852963893d4a84b677dfdf9b`;
  archive ownership, permissions and timestamps are deterministic.
- NOT RUN LOCALLY: Go race detector. The Windows host has no C toolchain; the
  immutable Linux CI workflow retains the race gate.
- NOT APPLICABLE YET: GoReleaser's SCM-aware `check` cannot resolve release
  refs because this local repository has neither a first commit nor a remote.
  The complete snapshot build, tests, archive, SBOM and checksums pass.
- NOT RUN: Debian 13 and Ubuntu 24.04 VM jobs, because no CI remote has been
  activated and x1 is not a disposable test VM.
- SKIPPED BY OWNER: interactive Codex Security scan. This is an explicit
  omission, not a passing result. Deterministic local scans above still ran.

## Documentation deployment

- The English Astro/Starlight guide is live at
  `https://yamsplus-guide.vercel.app`. Vercel reported production deployment
  `dpl_3Brb49T1mYAZkX6CxJMjGiVMkyhd` as `READY` on 2026-08-06.
- The initial no-`--prod` deployment was nevertheless assigned Vercel's
  production target and stable `vercel.app` alias for the new project. No
  custom domain was attached and no domain was purchased.
- All nine guide routes returned HTTP 200, the deliberate missing route
  returned 404, the sitemap contains no `example.invalid` URLs, and 14 local
  Playwright link, navigation, accessibility and analytics tests passed
  against the production build before deployment.
- No affiliate URLs are enabled. The public guide, but not YAMS Plus or an
  installed media stack, loads the self-hosted Umami instance at
  `umami.arturf.ch` with website ID
  `f96b52f3-0218-4a31-ba34-0c3753efa7d8`. Tracking is restricted to
  `yamsplus-guide.vercel.app`, honours Global Privacy Control and Do Not Track,
  and strips unsafe URL data. Bounded Reddit campaign parameters continue only
  across internal guide calls to action; deeper guide pages emit no custom
  landing-page interaction events. The live production HTML was checked after
  deployment and contains the expected domain-scoped tracker.

## Blocking container scan

Trivy 0.73.0 scanned every immutable image at High/Critical. Counts below are
finding occurrences and may repeat a CVE across installed copies of a package.
The hard CI gate remains enabled; no blanket ignore list was added.

| Locked image | High | Critical |
| --- | ---: | ---: |
| Jellyfin | 45 | 5 |
| Seerr | 80 | 4 |
| Radarr | 40 | 0 |
| Sonarr | 7 | 0 |
| Prowlarr | 40 | 0 |
| Bazarr | 2 | 0 |
| SABnzbd | 6 | 0 |
| qBittorrent | 0 | 0 |
| Gluetun | 15 | 0 |
| Recyclarr | 0 | 0 |
| Audiobookshelf | 61 | 4 |
| Shelfmark | 578 | 46 |

The latest stable Seerr image, for example, contains fixable Critical findings
in `handlebars` and `tar`. The candidate must not be described as security-gate
green until upstream images are rebuilt or each remaining finding is narrowly
assessed and resolved.

## Outstanding acceptance

- No authorised film or series was supplied, so no media was requested or
  downloaded.
- No Prowlarr indexer was added.
- Plugin API loading is proven locally; the Editor's Choice shelf, two-episode
  Intro Skipper fixture, playback report entry, metadata searches and normal
  user request flow require the x1 browser/media scenario.
- Encrypted YAMS Plus backup/isolated restore and host reboot remain to be
  demonstrated. A separate encrypted legacy-migration rollback archive was
  created and verified at
  `/var/backups/yamsplus-migration/legacy-yams-20260805T083348Z.tar.gz.enc`;
  it is mode `0600` and its key is root-only.
- x1 was rechecked and prepared live on 2026-08-05. It is Ubuntu 24.04/amd64
  with 8 logical CPUs, 15 GiB RAM, Docker 29.6.1 and Compose 5.2.0. Targeted
  deletion of large Trash items, Snap/APT caches, Docker build cache, dangling
  images, old journals and the stopped 35 GiB Zen build container increased
  root free space from 27 GiB (90% used) to approximately 91 GiB (65% used).
  Small trashed documents and all unrelated project data were retained.
- `/etc/yamsplus`, `/opt/yamsplus`, `/var/lib/yamsplus` and `/srv/yamsplus` now
  contain the prepared desired state. The installed binary SHA-256 is
  `5ce1ea0d7b712ca362fef2a6912d7631a70add6884d43c2f17c79076d0c95120`,
  byte-identical to the final snapshot binary. `plan --json` reports all five
  generated files unchanged. Three earlier binaries remain root-only under
  `/var/backups/yamsplus-migration/` for rollback.
- The production preflight is green for OS, architecture, Docker, Compose,
  CPU, RAM, ext4 storage, time sync, DNS, Intel DRM GPU, Tailscale and the
  selected LAN/Tailscale-only bind addresses. Shared app and data paths are
  live-verified as `artur:artur`, with media/download roots at mode `0770`.
- The legacy media link `/home/artur/yams-media` points to the currently absent
  `/media/artur/hdd-storage6/yams`; that storage is not present in `lsblk` or
  `findmnt`. `/srv/yamsplus` on the internal ext4 filesystem is therefore the
  explicit beta-test target; its roughly 90 GiB headroom is not presented as
  long-term media capacity.
- The legacy `portainer`, `prowlarr`, `seerr` and `gluetun` containers were
  stopped after the verified rollback archive was created. Their files and
  stopped containers were retained. The separate `ckw` project remains running
  and was not modified.
- Both legacy Newshosting configurations fail verified NNTP TLS authentication
  with server response `502 Authentication Failed`; no invalid credential was
  copied. The legacy Bazarr configuration contains no OpenSubtitles username or
  password. `sudo yamsplus apply` is intentionally paused until the owner enters
  valid provider credentials and the shared admin password through hidden input.
- The disposable smoke stack started nine containers, exposed only alternate
  `127.0.0.1` ports, proved the full configuration graph, then was removed with
  its temporary volumes and dummy secrets. That generated test state is not
  recoverable and contained no real credentials or media. The prepared
  production project still has zero running containers and its configuration
  hashes are unchanged.
- No media was requested or downloaded: Seerr requests, SABnzbd queue, download
  files and library files were all zero. No code was pushed, no CLI release was
  published, and no custom domain was registered or activated. Only the guide
  deployment described above is public.
