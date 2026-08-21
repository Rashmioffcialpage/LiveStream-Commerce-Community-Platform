"use client";

import { useEffect, useState } from "react";
import { ApiError, cancelSubscription, mySubscriptions, subscribeToChannel } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";

export function SubscribeButton({ channelId, slug }: { channelId: string; slug: string }) {
  const { token, user } = useAuth();
  const [subscriptionId, setSubscriptionId] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!token) return;
    mySubscriptions(token)
      .then((subs) => {
        const active = subs.find((s) => s.channel_id === channelId && s.status === "active");
        setSubscriptionId(active?.id ?? null);
      })
      .catch(() => {});
  }, [token, channelId]);

  if (!user) return null;

  async function handleSubscribe() {
    if (!token) return;
    setBusy(true);
    setError(null);
    try {
      const sub = await subscribeToChannel(token, slug);
      setSubscriptionId(sub.id);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "could not subscribe");
    } finally {
      setBusy(false);
    }
  }

  async function handleCancel() {
    if (!token || !subscriptionId) return;
    setBusy(true);
    setError(null);
    try {
      await cancelSubscription(token, subscriptionId);
      setSubscriptionId(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "could not cancel");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div>
      {subscriptionId ? (
        <button
          onClick={handleCancel}
          disabled={busy}
          className="bg-surface-2 border border-border text-sm font-medium rounded-md px-3 py-1.5 disabled:opacity-50"
        >
          Subscribed -- cancel
        </button>
      ) : (
        <button
          onClick={handleSubscribe}
          disabled={busy}
          className="bg-accent text-black text-sm font-medium rounded-md px-3 py-1.5 disabled:opacity-50"
        >
          Subscribe -- $4.99/mo
        </button>
      )}
      {error && <p className="text-xs text-danger mt-1">{error}</p>}
    </div>
  );
}
