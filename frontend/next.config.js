/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
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
