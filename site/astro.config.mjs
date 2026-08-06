import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

const productionDomain = 'yamsplus-guide.vercel.app';
const defaultUmamiWebsiteId = 'f96b52f3-0218-4a31-ba34-0c3753efa7d8';
const umamiWebsiteId = process.env.PUBLIC_UMAMI_WEBSITE_ID === undefined
  ? defaultUmamiWebsiteId
  : process.env.PUBLIC_UMAMI_WEBSITE_ID.trim();
const umamiHostUrl = (process.env.PUBLIC_UMAMI_HOST_URL ?? 'https://umami.arturf.ch').replace(/\/+$/, '');
const analyticsHead = umamiWebsiteId ? [
  {
    tag: 'script',
    attrs: { src: '/analytics.js', defer: true },
  },
  {
    tag: 'script',
    attrs: {
      src: `${umamiHostUrl}/script.js`,
      defer: true,
      'data-website-id': umamiWebsiteId,
      'data-host-url': umamiHostUrl,
      'data-domains': productionDomain,
      'data-tag': 'public-guide',
      'data-do-not-track': 'true',
      'data-exclude-hash': 'true',
      'data-before-send': 'yamsPlusAnalyticsBeforeSend',
    },
  },
] : [];

export default defineConfig({
  site: `https://${productionDomain}`,
  integrations: [
    starlight({
      title: 'YAMS Plus',
      description: 'The Jellyfin stack that configures the boring bits for you.',
      favicon: '/favicon.svg',
      head: analyticsHead,
      customCss: ['./src/styles/custom.css'],
      editLink: { baseUrl: 'https://github.com/arturict/yams-plus/edit/main/site/' },
      sidebar: [
        { label: 'Start here', items: [
          { label: 'Welcome', link: '/' },
          { label: 'Guide privacy', slug: 'privacy' },
          { label: 'Requirements', slug: 'requirements' },
          { label: 'Install', slug: 'install' },
          { label: 'The wizard', slug: 'wizard' },
          { label: 'Your one manual step', slug: 'prowlarr' },
        ]},
        { label: 'Use it', items: [
          { label: 'Daily operations', slug: 'operations' },
          { label: 'Optional modules', slug: 'modules' },
          { label: 'Troubleshooting', slug: 'troubleshooting' },
          { label: 'Backup and recovery', slug: 'recovery' },
        ]},
      ],
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/arturict/yams-plus' },
      ],
    }),
  ],
});
