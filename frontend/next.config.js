/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  output: 'standalone',
  distDir: process.env.NEXT_DIST_DIR || '.next',
  // Next 14.0: sync native History API changes without a route navigation.
  experimental: {
    windowHistorySupport: true,
  },
  // Dev only: proxy /api/* → production API. Production (next start) serves /api from the same host.
  async rewrites() {
    if (process.env.NODE_ENV !== 'development') {
      return [];
    }
    return [
      {
        source: '/api/:path*',
        destination: 'https://diaoxinxin.com/api/:path*',
      },
    ];
  },
  webpack: (config) => {
    return config;
  },
}

module.exports = nextConfig
