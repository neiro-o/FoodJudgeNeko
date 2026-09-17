/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  distDir: process.env.NEXT_DIST_DIR || '.next',
  webpack: (config) => {
    return config;
  },
  async rewrites() {
    const apiTarget = process.env.API_PROXY_TARGET || 'https://diaoxinxin.com';
    return [
      {
        source: '/api/:path*',
        destination: `${apiTarget}/api/:path*`,
      },
    ];
  },
}

module.exports = nextConfig
