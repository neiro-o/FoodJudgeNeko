/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  distDir: process.env.NEXT_DIST_DIR || '.next',
  webpack: (config) => {
    return config;
  },
}

module.exports = nextConfig
