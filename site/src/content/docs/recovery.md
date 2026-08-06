---
title: Backup and recovery
description: Encrypted configuration backups, isolated restores and a calm way home.
---

Backups contain configuration, application databases, state and the version lock. Media is excluded by default.

```sh
sudo yamsplus backup --output yamsplus-$(date +%F).tar.gz.age
```

You enter a passphrase interactively. Because the archive contains provider and API secrets, it is encrypted before it leaves the process.

## Test a restore

Restore into an isolated root first:

```sh
sudo yamsplus --root /srv/yamsplus-restore-test restore --yes ./yamsplus-2026-08-04.tar.gz.age
sudo yamsplus --root /srv/yamsplus-restore-test doctor --files-only
```

When the isolated check is green, stop the production stack, take one more backup, then restore to the production root.

## Uninstall

```sh
sudo yamsplus uninstall --yes
```

This removes containers and YAMS Plus configuration while preserving `/srv/yamsplus`. Media deletion is a separate guarded workflow; the beta CLI refuses to do it casually.
