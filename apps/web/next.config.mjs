/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  output: "export",
  distDir: process.env.NH_MEDIA_NEXT_DIST_DIR || ".next",
};
export default nextConfig;
