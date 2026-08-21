"use client";

import { useEffect, useRef, useState } from "react";
import { listNotifications, markNotificationRead, notificationsWsUrl, type Notification } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";

export function NotificationBell() {
  const { token, user } = useAuth();
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [open, setOpen] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    if (!token) return;
    listNotifications(token)
      .then(setNotifications)
      .catch(() => {});

    const ws = new WebSocket(notificationsWsUrl(token));
    wsRef.current = ws;
    ws.onmessage = (evt) => {
      const n: Notification = JSON.parse(evt.data);
      setNotifications((prev) => {
        if (prev.some((existing) => existing.id === n.id)) return prev; // dedupe against the WS's own history replay
        return [n, ...prev];
      });
    };
    return () => ws.close();
  }, [token]);

  if (!user) return null;

  const unreadCount = notifications.filter((n) => !n.read_at).length;

  async function handleOpen() {
    setOpen((o) => !o);
  }

  async function handleMarkRead(id: string) {
    if (!token) return;
    try {
      await markNotificationRead(token, id);
      setNotifications((prev) => prev.map((n) => (n.id === id ? { ...n, read_at: new Date().toISOString() } : n)));
    } catch {
      // best-effort -- leave it unread in the UI if the request failed
    }
  }

  return (
    <div className="relative">
      <button onClick={handleOpen} className="relative text-muted hover:text-foreground text-sm">
        🔔
        {unreadCount > 0 && (
          <span className="absolute -top-1.5 -right-2 bg-danger text-white text-[10px] rounded-full px-1.5">
            {unreadCount}
          </span>
        )}
      </button>
      {open && (
        <div className="absolute right-0 mt-2 w-80 bg-surface border border-border rounded-lg shadow-lg z-10 max-h-96 overflow-y-auto">
          {notifications.length === 0 && <p className="text-xs text-muted p-3">No notifications yet.</p>}
          {notifications.map((n) => (
            <button
              key={n.id}
              onClick={() => handleMarkRead(n.id)}
              className={`block w-full text-left p-3 border-b border-border text-xs hover:bg-surface-2 ${n.read_at ? "opacity-50" : ""}`}
            >
              <div className="font-medium text-foreground">{n.title}</div>
              <div className="text-muted mt-0.5">{n.body}</div>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
