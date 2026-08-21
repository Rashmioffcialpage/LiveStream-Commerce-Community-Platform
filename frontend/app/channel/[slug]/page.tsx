"use client";

import { use, useEffect, useState } from "react";
import { ApiError, createStream, endStream, getChannel, goLive, listChannelStreams, listSubscribers } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { useViewerCount } from "@/lib/use-viewer-count";
import { ChatPanel } from "@/components/ChatPanel";
import { PastStreamRow } from "@/components/PastStreamRow";
import { SubscribeButton } from "@/components/SubscribeButton";
import { GiftPanel } from "@/components/GiftPanel";
import type { Channel, Stream } from "@/lib/types";

export default function ChannelPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = use(params);
  const { user, token } = useAuth();

  const [channel, setChannel] = useState<Channel | null>(null);
  const [streams, setStreams] = useState<Stream[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [newTitle, setNewTitle] = useState("");
  const [busy, setBusy] = useState(false);

  const [subscriberCount, setSubscriberCount] = useState<number | null>(null);

  const liveStream = streams.find((s) => s.status === "live") ?? null;
  const isOwner = !!(user && channel && user.id === channel.creator_id);
  const viewerCount = useViewerCount(liveStream?.id ?? null);

  useEffect(() => {
    if (!isOwner || !token) return;
    function refreshSubscriberCount() {
      listSubscribers(token!, slug)
        .then((subs) => setSubscriberCount(subs.length))
        .catch(() => {});
    }
    refreshSubscriberCount();
    // polled, not just fetched once, so a new subscriber shows up here
    // without a manual page reload -- same 5s cadence as the stream/
    // channel refresh() below.
    const interval = setInterval(refreshSubscriberCount, 5000);
    return () => clearInterval(interval);
  }, [isOwner, token, slug]);

  async function refresh() {
    try {
      const [c, s] = await Promise.all([getChannel(slug), listChannelStreams(slug)]);
      setChannel(c);
      setStreams(s);
    } catch {
      setError("channel not found");
    }
  }

  useEffect(() => {
    refresh();
    const interval = setInterval(refresh, 5000); // picks up status changes made from elsewhere
    return () => clearInterval(interval);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [slug]);

  async function handleScheduleAndGoLive(e: React.FormEvent) {
    e.preventDefault();
    if (!token || !newTitle) return;
    setBusy(true);
    try {
      const stream = await createStream(token, slug, newTitle, []);
      await goLive(token, stream.id);
      setNewTitle("");
      await refresh();
    } catch (err) {
      alert(err instanceof ApiError ? err.message : "could not go live");
    } finally {
      setBusy(false);
    }
  }

  async function handleEndStream() {
    if (!token || !liveStream) return;
    setBusy(true);
    try {
      await endStream(token, liveStream.id);
      await refresh();
    } catch (err) {
      alert(err instanceof ApiError ? err.message : "could not end stream");
    } finally {
      setBusy(false);
    }
  }

  if (error) return <p className="max-w-4xl mx-auto px-4 py-8 text-danger text-sm">{error}</p>;
  if (!channel) return <p className="max-w-4xl mx-auto px-4 py-8 text-muted text-sm">loading...</p>;

  return (
    <div className="max-w-6xl mx-auto px-4 py-8">
      <div className="flex items-start justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold">{channel.name}</h1>
          {channel.category && <p className="text-xs text-muted mt-1">{channel.category}</p>}
          {channel.description && <p className="text-sm text-muted mt-2">{channel.description}</p>}
          {isOwner && subscriberCount !== null && (
            <p className="text-xs text-accent mt-2">{subscriberCount} subscriber{subscriberCount === 1 ? "" : "s"}</p>
          )}
        </div>
        <div className="text-right flex flex-col items-end gap-2">
          {liveStream && (
            <div>
              <span className="bg-danger text-white text-xs font-semibold px-2 py-1 rounded">LIVE</span>
              <div className="text-sm text-muted mt-1">{viewerCount} watching</div>
            </div>
          )}
          {!isOwner && <SubscribeButton channelId={channel.id} slug={channel.slug} />}
        </div>
      </div>

      {isOwner && (
        <div className="bg-surface-2 border border-border rounded-lg p-4 mb-6">
          {liveStream ? (
            <div className="flex items-center justify-between">
              <span className="text-sm">
                Live: <span className="font-medium">{liveStream.title}</span>
              </span>
              <button
                onClick={handleEndStream}
                disabled={busy}
                className="bg-danger text-white text-sm font-medium rounded-md px-3 py-1.5 disabled:opacity-50"
              >
                End stream
              </button>
            </div>
          ) : (
            <form onSubmit={handleScheduleAndGoLive} className="flex gap-2">
              <input
                value={newTitle}
                onChange={(e) => setNewTitle(e.target.value)}
                placeholder="Stream title"
                required
                className="flex-1 bg-surface border border-border rounded-md px-3 py-1.5 text-sm"
              />
              <button
                type="submit"
                disabled={busy}
                className="bg-accent text-black text-sm font-medium rounded-md px-3 py-1.5 disabled:opacity-50"
              >
                Go live
              </button>
            </form>
          )}
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-[1fr_360px] gap-6">
        <div className="bg-surface border border-border rounded-lg aspect-video flex items-center justify-center text-muted text-sm">
          {liveStream ? (
            <div className="text-center">
              <div className="text-lg text-foreground mb-1">{liveStream.title}</div>
              <div>video player goes here -- see Task 3&apos;s scope note on WebRTC/SFU</div>
            </div>
          ) : (
            <div>offline</div>
          )}
        </div>

        <div>
          {liveStream ? (
            <ChatPanel streamId={liveStream.id} isLive={true} />
          ) : (
            <div className="bg-surface border border-border rounded-lg p-4 text-sm text-muted">
              Chat opens when this channel goes live.
            </div>
          )}
          <GiftPanel slug={slug} isOwner={isOwner} />
        </div>
      </div>

      {streams.filter((s) => s.status === "ended").length > 0 && (
        <div className="mt-8">
          <h2 className="text-sm font-medium text-muted mb-3">Past streams</h2>
          <div className="flex flex-col gap-3">
            {streams
              .filter((s) => s.status === "ended")
              .map((s) => (
                <PastStreamRow key={s.id} stream={s} isOwner={isOwner} onUploaded={refresh} />
              ))}
          </div>
        </div>
      )}
    </div>
  );
}
