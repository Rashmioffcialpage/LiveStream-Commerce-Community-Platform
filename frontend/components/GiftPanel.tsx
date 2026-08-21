"use client";

import { useEffect, useState } from "react";
import { ApiError, GIFT_CATALOG, buyCoins, getWallet, sendGift } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";

const GIFT_EMOJI: Record<string, string> = { rose: "🌹", heart: "❤️", diamond: "💎", rocket: "🚀" };

export function GiftPanel({ slug, isOwner }: { slug: string; isOwner: boolean }) {
  const { token, user } = useAuth();
  const [balance, setBalance] = useState<number | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string | null>(null);

  function refreshWallet() {
    if (!token) return;
    getWallet(token)
      .then((w) => setBalance(w.coin_balance))
      .catch(() => {});
  }

  useEffect(refreshWallet, [token]);

  if (!user || isOwner) return null;

  async function handleBuyCoins() {
    if (!token) return;
    setBusy(true);
    setMessage(null);
    try {
      const wallet = await buyCoins(token, 500);
      setBalance(wallet.coin_balance);
      setMessage("Bought 500 coins ($5.00)");
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "purchase failed");
    } finally {
      setBusy(false);
    }
  }

  async function handleSendGift(giftType: string) {
    if (!token) return;
    setBusy(true);
    setMessage(null);
    try {
      await sendGift(token, slug, giftType);
      setMessage(`Sent ${GIFT_EMOJI[giftType]} ${giftType}!`);
      refreshWallet();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "could not send gift");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="bg-surface border border-border rounded-lg p-3 mt-3">
      <div className="flex items-center justify-between mb-2">
        <span className="text-xs text-muted">Coins: {balance ?? "..."}</span>
        <button onClick={handleBuyCoins} disabled={busy} className="text-xs text-accent hover:underline disabled:opacity-50">
          Buy 500 coins ($5.00)
        </button>
      </div>
      <div className="flex gap-2">
        {Object.entries(GIFT_CATALOG).map(([type, cost]) => (
          <button
            key={type}
            onClick={() => handleSendGift(type)}
            disabled={busy}
            title={`${cost} coins`}
            className="flex-1 bg-surface-2 border border-border rounded-md py-1.5 text-center hover:border-accent disabled:opacity-50"
          >
            <div className="text-lg">{GIFT_EMOJI[type]}</div>
            <div className="text-[10px] text-muted">{cost}</div>
          </button>
        ))}
      </div>
      {message && <p className="text-xs text-muted mt-2">{message}</p>}
    </div>
  );
}
