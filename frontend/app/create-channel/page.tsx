"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { ApiError, createChannel } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";

export default function CreateChannelPage() {
  const router = useRouter();
  const { user, token, loading } = useAuth();

  const [slug, setSlug] = useState("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [category, setCategory] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  if (!loading && (!user || user.role !== "creator")) {
    return <p className="max-w-lg mx-auto mt-16 px-4 text-muted text-sm">Sign in as a creator to create a channel.</p>;
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!token) return;
    setError(null);
    setBusy(true);
    try {
      const channel = await createChannel(token, slug, name, description, category);
      router.push(`/channel/${channel.slug}`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "something went wrong");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="max-w-lg mx-auto mt-12 px-4">
      <h1 className="text-xl font-semibold mb-6">Create a channel</h1>
      <form onSubmit={submit} className="flex flex-col gap-3">
        <input
          placeholder="URL slug (e.g. your-channel)"
          value={slug}
          onChange={(e) => setSlug(e.target.value.toLowerCase())}
          pattern="[a-z0-9][a-z0-9\-]{1,48}[a-z0-9]"
          required
          className="bg-surface border border-border rounded-md px-3 py-2 text-sm"
        />
        <input
          placeholder="Channel name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
          className="bg-surface border border-border rounded-md px-3 py-2 text-sm"
        />
        <input
          placeholder="Category (e.g. tech, gaming)"
          value={category}
          onChange={(e) => setCategory(e.target.value)}
          className="bg-surface border border-border rounded-md px-3 py-2 text-sm"
        />
        <textarea
          placeholder="Description"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          rows={3}
          className="bg-surface border border-border rounded-md px-3 py-2 text-sm"
        />
        {error && <p className="text-danger text-sm">{error}</p>}
        <button
          type="submit"
          disabled={busy}
          className="bg-accent text-black font-medium rounded-md px-3 py-2 text-sm mt-2 disabled:opacity-50"
        >
          {busy ? "..." : "Create channel"}
        </button>
      </form>
    </div>
  );
}
