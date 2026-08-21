import type { NextConfig } from "next";
import path from "path";

const nextConfig: NextConfig = {
  // this app lives in a subdirectory of a monorepo-shaped layout
  // (services/* alongside frontend/), not its own git repo root --
  // without this Turbopack falls back to guessing and warns about it.
  turbopack: {
    root: path.join(__dirname),
  },
};

export default nextConfig;
