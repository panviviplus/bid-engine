import remarkGfm from "remark-gfm";
import rehypeSlug from "rehype-slug";
import createMDX from "@next/mdx";

/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: false,
  // 允许在本地开发服务运行时使用独立目录执行生产构建检查，避免两个 Next 进程争用 .next。
  distDir: process.env.NEXT_DIST_DIR || ".next",
  output: "standalone",
  pageExtensions: ["js", "jsx", "ts", "tsx", "md", "mdx"],
  typescript: {
    ignoreBuildErrors: true,
  },
  eslint: {
    ignoreDuringBuilds: true,
  },
  webpack: (config) => {
    config.resolve.alias.canvas = false;

    return config;
  },
  async redirects() {
    return [
      // {
      //   source: "/",
      //   destination: "/file-analysis",
      //   permanent: false,
      // },
    ];
  },
  async rewrites() {
    return [];
  },
};

const withMDX = createMDX({
  extension: /\.mdx?$/,
  options: {
    remarkPlugins: [remarkGfm],
    rehypePlugins: [rehypeSlug],
    providerImportSource: "@mdx-js/react",
  },
});

export default withMDX(nextConfig);
