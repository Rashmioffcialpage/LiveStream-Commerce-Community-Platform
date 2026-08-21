"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { listChannels } from "@/lib/api";
import type { Channel } from "@/lib/types";

export default function HomePage() {
  const [channels, setChannels] = useState<Channel[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    listChannels()
      .then(setChannels)
      .catch(() => setError("could not load channels -- is stream-service running?"));
  }, []);

  return (
    <div className="max-w-4xl mx-auto px-4 py-8">
      <h1 className="text-xl font-semibold mb-6">Channels</h1>

      {error && <p className="text-danger text-sm">{error}</p>}
      {!error && channels === null && <p className="text-muted text-sm">loading...</p>}
      {channels?.length === 0 && (
        <p className="text-muted text-sm">
          No channels yet.{" "}
          <Link href="/login" className="text-accent hover:underline">
            Sign up as a creator
          </Link>{" "}
          to create one.
        </p>
      )}

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {channels?.map((c) => (
          <Link
            key={c.id}
            href={`/channel/${c.slug}`}
            className="block bg-surface border border-border rounded-lg p-4 hover:border-accent transition-colors"
          >
            <div className="font-medium">{c.name}</div>
            {c.category && <div className="text-xs text-muted mt-1">{c.category}</div>}
            {c.description && <p className="text-sm text-muted mt-2 line-clamp-2">{c.description}</p>}
          </Link>
        ))}
      </div>
    </div>
  );
}
