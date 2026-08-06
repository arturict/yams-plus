---
title: Guide privacy
description: What the public YAMS Plus guide measures, and what remains private.
---

## The installed stack has no analytics

YAMS Plus, its command-line tool and the Jellyfin, Seerr and Arr services it installs do not contain this tracker. Analytics run only on the public guide at `yamsplus-guide.vercel.app`.

## What the guide measures

The guide uses a self-hosted [Umami](https://umami.is/) instance at `umami.arturf.ch`. It records anonymous page views. On the landing page only, it also records a small set of interactions:

- clicks on the install, exploration and selected navigation actions;
- first views of the hero, overview and included-features sections;
- scroll milestones at 25, 50, 75 and 100 percent; and
- active time milestones at 30, 60 and 120 seconds.

Event data uses only fixed categories such as `hero`, `install-guide` or a numeric threshold. It never includes link labels, query strings, command contents, search text, form values, media-library data, credentials or an account identifier.

## Campaign attribution

For links shared on Reddit or elsewhere, the guide accepts the five standard campaign fields: `utm_source`, `utm_medium`, `utm_campaign`, `utm_content` and `utm_term`. These fields are kept on internal guide links so a visit from the landing page through the install instructions can be measured as one campaign journey. Umami reads them natively from the URL; they are not copied into custom event data and are never added to outbound links.

Before analytics are sent, every other query parameter and the URL fragment are removed. Campaign values accept only a bounded 64-character token alphabet. The tracker sends nothing when the browser enables Global Privacy Control or Do Not Track, and it is restricted to the production guide domain.

Umami is configured without analytics cookies or cross-site identification. No analytics code is loaded when the guide is built with an empty `PUBLIC_UMAMI_WEBSITE_ID`.
