---
title: Your one manual step
description: Add an indexer, press test, and hand control back to the installer.
---

YAMS Plus pauses after it connects Prowlarr to Radarr, Sonarr and Shelfmark. Open the displayed private Prowlarr URL.

1. Choose **Indexers → Add Indexer**.
2. Select an indexer you are permitted to use.
3. Enter its credentials and categories.
4. Press **Test**, then **Save**.
5. Return to the terminal and run:

```sh
sudo yamsplus doctor
```

Expected result:

```text
healthy          prowlarr-indexers  1 indexer(s) configured
```

`doctor` checks Prowlarr live. It does not merely trust a stale “done” checkbox. Full Sync then distributes the indexer to the selected Arr applications.

:::note[Why is this not automated?]
Indexer availability, terms and credentials are personal and change independently of YAMS Plus. Selecting one on your behalf would be both brittle and rather presumptuous.
:::
