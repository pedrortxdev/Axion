import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  async rewrites() {
    return [
      {
        source: '/cluster/:path*',
        destination: 'http://127.0.0.1:8500/cluster/:path*', // Proxy cluster requests to Go backend
      },
      // Add other rewrites here if needed, like:
      // {
      //   source: '/api/:path*',
      //   destination: 'http://127.0.0.1:8500/api/:path*',
      // },
      // {
      //   source: '/instances/:path*',
      //   destination: 'http://127.0.0.1:8500/instances/:path*',
      // },
    ];
  },
};

export default nextConfig;
