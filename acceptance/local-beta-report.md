# Beta candidate evidence

Three runs are recorded here. The 2026-09-22 run is the current state; the
2026-09-21 run and the original 2026-08-07 beta-candidate evidence stay below,
because the container-scan comparison depends on them.

# Run 3 — 2026-09-22

Same host, against the branch after a release-readiness audit and its fixes.
Isolated roots under `/tmp`, separate Compose projects (`ypaudit`,
`ypaudit2`), loopback-only alternate ports, placeholder provider credentials,
no indexer, no media, a random admin password never printed. The prepared
production install under `/etc/yamsplus` and `/opt/yamsplus` was not touched.

## Torrent install, never run end to end before

Movies, series and subtitles with qBittorrent and no VPN. Run 2 exercised only
the Usenet shape live, so its "clean second apply" said nothing about torrent,
and this run found two defects before it could get to the one it was meant to
check:

- **qBittorrent 5.2 logins failed.** The lock moved qBittorrent to 5.2.3,
  which answers a successful login with `204` and no body; the client accepted
  only `Ok.`. Separately, until host-header validation is turned off,
  qBittorrent refuses a Host port other than its own WebUI port, so a
  published port other than 8081 got `401`. Reproduced by hand: `401` through
  the remapped port, `204` with the container port, `204` through the
  remapped port once validation was off. Both fixed.
- **Seerr refused every apply after the first** with `403 invalid csrf token`,
  because convergence had turned on Seerr's `csrfProtection`, which Seerr
  describes as "set external API access to read-only (requires HTTPS)". Its
  CSRF cookies are `Secure`, so a browser on a plain-HTTP LAN or Tailscale
  address would not return them either. It is now left off, switched off where
  it was on (one Seerr restart), and the write is no longer error-swallowed.
  On a stack that already had it on: the first re-apply turned it off and
  restarted Seerr, the second ran clean, API-key writes succeed.

With those fixed, from a fresh root:

| Step | Result |
| --- | --- |
| Install | exit 0 |
| Second apply with the pre-fix password logic (built from the parent of that fix plus the qBittorrent fix), no password | exit 1: `qBittorrent login failed … 401 … temporary WebUI password was not found` |
| Second apply with the fix | asked once for the password, exit 0 |
| Third apply | exit 0, no Seerr restart |
| `doctor --json` | 18 checks, 0 failed, 1 action required (the Prowlarr indexer) |
| Bazarr `analytics.enabled` | `false` (the image defaults to `true`) |
| Seerr `csrfProtection` | `false` |
| qBittorrent login through the published port | admin password `204`, empty password `401` |

## Usenet install with Books

| Step | Result |
| --- | --- |
| Install | exit 0 |
| Second apply with no password and no terminal | exit 0, zero password prompts |
| `backup` of the running stack | stopped and restarted all nine running services, exit 0 |
| `restore` into an empty root | `yamsplus.yaml`, `compose.yaml` and the lock byte-identical; secrets `0600` in a `0750` directory; wrong passphrase refused |

The archive has no `jellyfin.db-wal` or `-shm`: Jellyfin was stopped and
checkpointed before the copy. Every file the source had and the restore did
not was written after the backup finished.

A `doctor` run five seconds after the backup reported four endpoints down while
the services were still booting. Measured separately: `doctor` 0 failed before,
the backup took 16 seconds, and `doctor` was at 0 failed again 19 seconds
after it returned. The stack is briefly unavailable during a backup, as the
recovery guide now says.

## Automated gates

`go test -race ./...`, `go vet` for Linux and Windows, an arm64 build,
Staticcheck v0.7.0 and Gitleaks green. The `cmd/yamsplus`, `internal/stack`
and `internal/backup` tests also pass as root, locally and in CI's new root
step.

## Still not proven

- The VPN shapes (Gluetun) live, which need VPN credentials.
- The authorised media path, the UI-level plugin checks and the Debian 13 and
  Ubuntu 24.04 VM scenarios, as before.
- The container scan, as before.

# Run 2 — 2026-09-21

Ubuntu 24.04 / amd64 host (`x1`, 8 logical CPUs, 15 GiB RAM, Docker 29.8.1,
Compose 5.5.1), against the branch that recovers the unpublished worktree
fixes, fixes four further defects and refreshes the image lock. Every run used
an isolated install root, a separate Compose project, loopback-only alternate
ports, placeholder provider credentials, no indexer and no media. The prepared
production install under `/etc/yamsplus` was not touched and still has zero
running containers.

## What was proven

- Full-module install (movies, series, subtitles, Books) converged through the
  application APIs. `doctor --json`: **22 checks, 0 failed, 1 action required**,
  the action being the intentional manual Prowlarr indexer step.
- `plugins audit --json`: all 13 required compatible plugins active.
- A second `apply` with no admin password and no provider credentials in the
  environment reported all five generated files `unchanged`.
- Encrypted backup and restore into a separate empty root: the archive is
  `age-encryption.org/v1` scrypt, restored `yamsplus.yaml` and `compose.yaml`
  are byte-identical to the source, secrets land at mode `0600` inside a `0750`
  root-only directory, and a wrong passphrase is refused.
- `stop` then `start`: 22 checks, 0 failed again.
- `uninstall --yes` removed the containers and the three managed directories
  and preserved the media root.
- All five downloader shapes rendered and passed `docker compose config`, with
  no floating `latest` tag, no Docker socket mount and no privileged container.
- **Host independence**: the same binary rendered all five shapes on Ubuntu
  24.04 (`x1`) and Ubuntu 26.04 (`dev-t15`) into the same root path. All 28
  generated files are byte-identical across the two hosts and the recorded
  config digests match; only `state.json` differs, in its `updatedAt` stamp.
- The refreshed image lock was installed end to end on Radarr 6.4.4, Sonarr
  4.0.20, Prowlarr 2.6.5, SABnzbd 5.1.3, Bazarr 1.6.1, Recyclarr 8.7.2,
  Audiobookshelf 2.36.1 and Shelfmark 1.3.15: 22 checks / 0 failed / 1 action
  required, 13 of 13 plugins active, a warning-free Recyclarr sync and a clean
  second apply.

## Automated gates

- PASS: `go test -race ./...`, `go vet ./...`, Staticcheck v0.7.0, Gitleaks.
- PASS: govulncheck v1.6.0 on Go 1.26.8 — zero vulnerabilities your code calls.
  One module-level advisory remains and cannot be cleared: GO-2026-5932,
  `golang.org/x/crypto/openpgp` is unmaintained, fixed in N/A, not reachable
  from this code.
- PASS: guide `npm ci`, `npm run build` (astro check: 0 errors) and 14 of 14
  Playwright link, navigation, accessibility and analytics tests.
- `npm audit`: 0 High, 0 Critical, 1 moderate (`devalue` <5.9.1, transitive).
- ARM64 **compiles** (`GOOS=linux GOARCH=arm64 go build` succeeds). It is not
  shipped because `.goreleaser.yaml` builds only `linux-amd64`, the lock
  records `architecture: amd64`, and preflight hard-fails off amd64.

## Blocking container scan, rescanned

Trivy 0.73.0 at High/Critical against the twelve digests the lock pinned in
August — byte-identical images, so the whole delta is the vulnerability
database catching up, not a change in the stack.

| Locked image | High Aug → Sep | Critical Aug → Sep |
| --- | --- | --- |
| Jellyfin | 45 → 123 | 5 → 4 |
| Seerr | 80 → 104 | 4 → 6 |
| Radarr | 40 → 52 | 0 → 0 |
| Sonarr | 7 → 11 | 0 → 0 |
| Prowlarr | 40 → 52 | 0 → 0 |
| Bazarr | 2 → 63 | 0 → 0 |
| SABnzbd | 6 → 29 | 0 → 0 |
| qBittorrent | 0 → 23 | 0 → 0 |
| Gluetun | 15 → 26 | 0 → 0 |
| Recyclarr | 0 → 14 | 0 → 0 |
| Audiobookshelf | 61 → 83 | 4 → 4 |
| Shelfmark | 578 → 930 | 46 → 49 |
| **Total** | **874 → 1510** | **59 → 63** |

Nothing improved. The two images August could call clean, qBittorrent and
Recyclarr at 0/0, are now 23 and 14 High. Shelfmark dominates: `chromium` and
`chromium-common` alone account for 614 of its 930 High and 42 of its 49
Critical findings, from a base image that is several Debian chromium releases
behind. Radarr's and Prowlarr's 52 are nine unique .NET runtime CVEs counted
once per installed copy. The Seerr `handlebars` and `tar` Critical findings the
August report named are still present and still fixable.

This candidate must not be called security-gate green. That conclusion is
unchanged from August and the numbers are worse.

## Outstanding acceptance

Unchanged from August unless stated:

- No authorised film or series was supplied, so no media was requested or
  downloaded, and no Prowlarr indexer was added. Both legacy Newshosting
  configurations on this host still fail NNTP TLS authentication with `502
  Authentication Failed`, so the download path cannot be proven without valid
  provider credentials from the owner.
- The UI-level plugin scenario (Editor's Choice shelf, Intro Skipper two-episode
  fixture, playback report entry, metadata searches, a normal user request)
  still needs a browser session against a real library.
- Disposable Debian 13 and Ubuntu 24.04 VM scenarios are still not run. The
  Ubuntu 26.04 render-parity check above is evidence of host independence for
  the generated files, not a substitute for a full install on a second distro.
- No signed or tagged CLI release, and no ARM64 artifact.
- **New**: Jellyfin cannot be moved past 10.11.11 yet. Jellyfin 12 disables
  legacy authorization by default, which retires the `X-Emby-Token` header the
  convergence code authenticates with, and the required `Custom Tabs` plugin has
  no Jellyfin 12 build.

# Run 1 — 2026-08-07

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

- PASS: `go test -race ./...`, `go vet ./...`, Staticcheck v0.7.0 on the
  public Ubuntu 24.04 GitHub Actions runner.
- PASS: govulncheck v1.6.0 with Go 1.26.5, zero reachable vulnerabilities.
- PASS: npm audit, Astro check/build and 10 Playwright accessibility/smoke tests.
- PASS: Gitleaks with runtime test data excluded by the checked-in policy.
- PASS: Trivy 0.73.0 repository scan at High/Critical after upgrading
  `golang.org/x/crypto` to v0.52.0.
- PASS: snapshot archive, SHA-256 checksums and SPDX 2.3 SBOM generation.
  Two clean builds produced the identical archive SHA-256
  `5fe0a278a2da02aa30320cbfd436b74d0cd7060c852963893d4a84b677dfdf9b`;
  archive ownership, permissions and timestamps are deterministic.
- PASS: the public CI workflow independently validated all five downloader
  shapes, the image-lock matrix, the guide, Gitleaks and the High/Critical
  filesystem scan. Container-image findings remain isolated in the separate,
  still-blocking Container security workflow rather than being hidden behind a
  green code-quality badge.
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
  `dpl_DNu1gJEpj7aMTAN43uk1Qdj9zmGZ` as `READY` and promoted it on 2026-08-07.
  The deployed canonical/OpenGraph URL, public GitHub link and source-install
  command were checked before promotion.
- The initial no-`--prod` deployment was nevertheless assigned Vercel's
  production target and stable `vercel.app` alias for the new project. No
  custom domain was attached and no domain was purchased.
- All guide routes returned HTTP 200 (nine documentation pages plus the
  landing page), the deliberate missing route
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
  files and library files were all zero. The GPL-3.0 source is public at
  `https://github.com/arturict/yams-plus`; no tagged CLI release was published
  and no custom domain was registered or activated.
