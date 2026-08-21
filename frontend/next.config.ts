import type { NextConfig } from "next";
import path from "path";

const nextConfig: NextConfig = {
  // this app lives in a subdirectory of a monorepo-shaped layout
  // (services/* alongside frontend/), not its own git repo root --
  // without this Turbopack falls back to guessing and warns about it.
  turbopack: {
    root: path.join(__dirname),
  },
  // a self-contained server bundle (node_modules pruned to only what's
  // actually imported) -- what the Docker image runs, so the image isn't
  // shipping the entire dev-time node_modules tree.
  output: "standalone",
};

export default nextConfig;
