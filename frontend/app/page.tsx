"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { listChannels, searchChannels, type SearchResult } from "@/lib/api";
import type { Channel } from "@/lib/types";

export default function HomePage() {
  const [channels, setChannels] = useState<Channel[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[] | null>(null);
  const [searching, setSearching] = useState(false);

  useEffect(() => {
    listChannels()
      .then(setChannels)
      .catch(() => setError("could not load channels -- is stream-service running?"));
  }, []);

  useEffect(() => {
    if (!query.trim()) {
      setResults(null);
      return;
    }
    const timeout = setTimeout(() => {
      setSearching(true);
      searchChannels(query)
        .then(setResults)
        .catch(() => setResults([]))
        .finally(() => setSearching(false));
    }, 300); // debounce -- one request per pause in typing, not per keystroke
    return () => clearTimeout(timeout);
  }, [query]);

  return (
    <div className="max-w-4xl mx-auto px-4 py-8">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-semibold">Channels</h1>
        <input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search by creator, category, stream title, or tag..."
          className="w-96 bg-surface border border-border rounded-md px-3 py-1.5 text-sm"
        />
      </div>

      {error && <p className="text-danger text-sm">{error}</p>}

      {query.trim() ? (
        <>
          {searching && <p className="text-muted text-sm">searching...</p>}
          {!searching && results?.length === 0 && <p className="text-muted text-sm">No results for &quot;{query}&quot;.</p>}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {results?.map((r) => (
              <Link
                key={r.id}
                href={`/channel/${r.slug}`}
                className="block bg-surface border border-border rounded-lg p-4 hover:border-accent transition-colors"
              >
                <div className="font-medium">{r.name}</div>
                <div className="text-xs text-muted mt-1">
                  {r.creator_name} {r.category && `· ${r.category}`}
                </div>
                {r.stream_titles.length > 0 && (
                  <p className="text-xs text-muted mt-2">Recent: {r.stream_titles[r.stream_titles.length - 1]}</p>
                )}
              </Link>
            ))}
          </div>
        </>
      ) : (
        <>
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
        </>
      )}
    </div>
  );
}
