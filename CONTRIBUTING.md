# Contributing to YAMS Plus

Thanks for helping make the installer less clicky and more predictable.

## Before you start

- Use YAMS Plus only with media, providers and indexers you are authorised to
  use.
- Open an issue before a broad architecture change or a new always-on service.
- Never include credentials, API keys, private addresses, provider account
  identifiers or real indexer details in an issue, test fixture or report.
- Keep the beta support matrix to Debian 13 and Ubuntu 24.04 on amd64 unless a
  change includes repeatable evidence for a new platform.

## Pull requests

1. Create a focused branch.
2. Add or update tests for behaviour changes.
3. Run `go test ./...`, `go vet ./...` and `make lint`. The lint rejects
   comments that start with TODO, FIXME, HACK, XXX or WORKAROUND: fix the
   problem or open an issue instead, and keep comments that explain why.
4. For guide changes, run `npm ci`, `npm run build` and `npm run test:e2e` in
   `site/`.
5. Explain the user-visible change, verification and any remaining limit in
   the pull request.

Application configuration must use supported APIs. Do not patch live SQLite
databases. Keep `Discover -> Plan -> Apply -> Verify` operations repeatable, and
ensure a second `apply` does not create duplicate clients, libraries or apps.

## Conduct

Be kind, specific and patient. Harassment, discrimination, personal attacks
and publishing another person's private information are not welcome. Project
maintainers may edit or remove contributions that violate these expectations.
