export type Recommendation = {
  name: string;
  officialUrl: string;
  affiliateUrl?: string;
  affiliateEnabled: boolean;
  disclosure: string;
};

// Affiliate activation is intentionally a content-only, reviewable change.
// Beta builds reject affiliateEnabled=true in the product configuration.
export const recommendations: Record<string, Recommendation> = {
  usenet: {
    name: 'Newshosting',
    officialUrl: 'https://www.newshosting.com/',
    affiliateEnabled: false,
    disclosure: 'No commission link is active. Other compatible NNTP providers work too.',
  },
  vpn: {
    name: 'Proton VPN',
    officialUrl: 'https://protonvpn.com/',
    affiliateEnabled: false,
    disclosure: 'No commission link is active. Generic WireGuard and OpenVPN providers are supported.',
  },
};
