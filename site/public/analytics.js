(() => {
  const campaignKeys = ['utm_source', 'utm_medium', 'utm_campaign', 'utm_content', 'utm_term'];
  const safeCampaignValue = /^[a-z0-9][a-z0-9._~-]{0,63}$/i;
  const privacySignalEnabled = () => navigator.globalPrivacyControl !== true
    && navigator.doNotTrack !== '1'
    && window.doNotTrack !== '1';

  const campaignData = (search = window.location.search) => {
    const params = new URLSearchParams(search);
    return Object.fromEntries(campaignKeys.flatMap(key => {
      const value = params.get(key)?.trim();
      return value && safeCampaignValue.test(value) ? [[key, value]] : [];
    }));
  };

  const privacySafeUrl = value => {
    const url = new URL(value, window.location.origin);
    const allowed = new URLSearchParams();
    const campaign = campaignData(url.search);
    for (const key of campaignKeys) {
      if (campaign[key]) allowed.set(key, campaign[key]);
    }
    const query = allowed.toString();
    return `${url.pathname}${query ? `?${query}` : ''}`;
  };

  window.yamsPlusAnalyticsBeforeSend = (_type, payload) => {
    if (!privacySignalEnabled()) return false;
    if (!payload || typeof payload !== 'object' || typeof payload.url !== 'string') return payload;
    return { ...payload, url: privacySafeUrl(payload.url) };
  };

  const preserveCampaignOnInternalLinks = () => {
    const campaign = campaignData();
    if (Object.keys(campaign).length === 0) return;

    for (const anchor of document.querySelectorAll('a[href]')) {
      const rawHref = anchor.getAttribute('href');
      if (!rawHref || rawHref.startsWith('#') || rawHref.startsWith('mailto:') || rawHref.startsWith('tel:')) continue;

      const destination = new URL(rawHref, window.location.href);
      if (destination.origin !== window.location.origin) continue;

      for (const [key, value] of Object.entries(campaign)) {
        if (!destination.searchParams.has(key)) destination.searchParams.set(key, value);
      }
      anchor.href = `${destination.pathname}${destination.search}${destination.hash}`;
    }
  };

  const locationFor = element => {
    if (element.closest('.hero')) return 'hero';
    if (element.closest('header')) return 'header';
    if (element.closest('nav')) return 'navigation';
    if (element.closest('main')) return 'content';
    return 'page';
  };

  const track = (name, data) => {
    if (privacySignalEnabled()) window.umami?.track(name, data);
  };

  const landingTargets = new Map([
    ['/install/', { action: 'install', target: 'install-guide' }],
    ['/wizard/', { action: 'explore', target: 'wizard-guide' }],
    ['/requirements/', { action: 'navigate', target: 'requirements-guide' }],
    ['/privacy/', { action: 'navigate', target: 'guide-privacy' }],
  ]);

  const trackLandingClick = event => {
    const target = event.target instanceof Element ? event.target : null;
    if (!target) return;

    const anchor = target.closest('a[href]');
    if (!anchor) return;

    const destination = new URL(anchor.href, window.location.href);
    if (destination.origin === window.location.origin) {
      const cta = landingTargets.get(destination.pathname);
      if (cta) track('landing-cta', { ...cta, location: locationFor(anchor) });
    } else if (destination.hostname === 'yams.media') {
      track('landing-cta', { action: 'outbound', location: locationFor(anchor), target: 'upstream-yams' });
    }
  };

  const startLandingMeasurement = () => {
    if (window.location.pathname !== '/') return;

    document.addEventListener('click', trackLandingClick);

    const viewedSections = new Set();
    const sections = [
      ['hero', document.querySelector('.hero')],
      ['overview', document.getElementById('one-wizard-one-manual-stop')],
      ['included', document.getElementById('what-you-get')],
    ];
    const sectionObserver = new IntersectionObserver(entries => {
      for (const entry of entries) {
        const section = entry.target.getAttribute('data-analytics-section');
        if (!entry.isIntersecting || !section || viewedSections.has(section)) continue;
        viewedSections.add(section);
        track('landing-section-view', { section });
        sectionObserver.unobserve(entry.target);
      }
    }, { threshold: 0.35 });
    for (const [section, element] of sections) {
      if (!element) continue;
      element.setAttribute('data-analytics-section', section);
      sectionObserver.observe(element);
    }

    const reachedDepths = new Set();
    const depthThresholds = [25, 50, 75, 100];
    const measureScrollDepth = () => {
      const scrollable = document.documentElement.scrollHeight - window.innerHeight;
      const depth = scrollable <= 0 ? 100 : Math.min(100, Math.round((window.scrollY / scrollable) * 100));
      for (const threshold of depthThresholds) {
        if (depth < threshold || reachedDepths.has(threshold)) continue;
        reachedDepths.add(threshold);
        track('landing-scroll-depth', { depth: threshold });
      }
    };
    window.addEventListener('scroll', measureScrollDepth, { passive: true });
    measureScrollDepth();

    const reachedTimes = new Set();
    const timeThresholds = [30, 60, 120];
    let engagedSeconds = 0;
    const timer = window.setInterval(() => {
      if (document.visibilityState !== 'visible' || !document.hasFocus()) return;
      engagedSeconds += 1;
      for (const threshold of timeThresholds) {
        if (engagedSeconds < threshold || reachedTimes.has(threshold)) continue;
        reachedTimes.add(threshold);
        track('landing-engaged-time', { seconds: threshold });
      }
      if (reachedTimes.size === timeThresholds.length) window.clearInterval(timer);
    }, 1000);
    window.addEventListener('pagehide', () => window.clearInterval(timer), { once: true });
  };

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => {
      preserveCampaignOnInternalLinks();
      startLandingMeasurement();
    }, { once: true });
  } else {
    preserveCampaignOnInternalLinks();
    startLandingMeasurement();
  }
})();
