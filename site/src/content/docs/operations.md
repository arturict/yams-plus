---
title: Daily operations
description: The small command set you will actually remember.
---

```sh
sudo yamsplus status
sudo yamsplus doctor --json
sudo yamsplus start
sudo yamsplus stop
sudo yamsplus restart
sudo yamsplus logs jellyfin
sudo yamsplus profiles sync
sudo yamsplus plugins audit
```

`status` is the human summary. `doctor --json` returns stable top-level states: `healthy`, `action-required` or `failed`, and never prints provider passwords or API keys.

## Change a module

Edit `/etc/yamsplus/yamsplus.yaml`, then inspect and apply:

```sh
sudo yamsplus plan
sudo yamsplus apply
```

The shared UI password is requested again because YAMS Plus intentionally does not store it. Missing provider credentials are requested in the same hidden-input flow; already configured values are not requested again. Internal per-service API tokens remain separate in `/etc/yamsplus/secrets/` with mode `0600`.

## Updates

`yamsplus update` uses the reviewed `stack.lock.yaml`; it does not chase `latest`. Review the new lock and release notes first, then run `sudo yamsplus update --yes`.
