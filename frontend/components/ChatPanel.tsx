"use client";

import { useEffect, useRef, useState } from "react";
import { chatHistory, chatWsUrl } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import type { ChatMessage } from "@/lib/types";

interface LiveMessage {
  type: string;
  id?: string;
  user_id?: string;
  display_name?: string;
  body?: string;
  created_at?: string;
  reason?: string;
  messages?: ChatMessage[];
}

const REACTIONS = ["👍", "❤️", "🔥", "😂", "😮", "🎉"];

export function ChatPanel({ streamId, isLive }: { streamId: string; isLive: boolean }) {
  const { token, user } = useAuth();
  const [messages, setMessages] = useState<LiveMessage[]>([]);
  const [connected, setConnected] = useState(false);
  const [input, setInput] = useState("");
  const [rejection, setRejection] = useState<string | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setMessages([]);
    if (!isLive) return;

    // logged-in: connect live over WebSocket, with history delivered as
    // the socket's first frame. Logged-out: read-only via REST -- chat
    // WS requires auth (a message needs a sender identity for
    // moderation), so there's no anonymous live path.
    if (token) {
      const ws = new WebSocket(chatWsUrl(streamId, token));
      wsRef.current = ws;
      ws.onopen = () => setConnected(true);
      ws.onclose = () => setConnected(false);
      ws.onmessage = (evt) => {
        const msg: LiveMessage = JSON.parse(evt.data);
        if (msg.type === "history") {
          setMessages(msg.messages ?? []);
        } else if (msg.type === "error") {
          setRejection(msg.reason ?? "message rejected");
          setTimeout(() => setRejection(null), 4000);
        } else if (msg.type === "message-deleted") {
          setMessages((prev) => prev.filter((m) => m.id !== msg.id));
        } else {
          setMessages((prev) => [...prev, msg]);
        }
      };
      return () => ws.close();
    } else {
      chatHistory(streamId)
        .then((history) => setMessages(history))
        .catch(() => {});
    }
  }, [streamId, token, isLive]);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  function send(type: "message" | "reaction", body: string) {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN || !body) return;
    wsRef.current.send(JSON.stringify({ type, body }));
  }

  return (
    <div className="flex flex-col h-[560px] bg-surface border border-border rounded-lg overflow-hidden">
      <div className="px-3 py-2 border-b border-border text-xs text-muted flex items-center justify-between">
        <span>Chat</span>
        {token && <span className={connected ? "text-accent" : ""}>{connected ? "live" : "connecting..."}</span>}
      </div>

      <div className="flex-1 overflow-y-auto px-3 py-2 flex flex-col gap-1.5 text-sm">
        {!isLive && <p className="text-muted text-xs">Chat opens once the stream is live.</p>}
        {isLive && messages.length === 0 && <p className="text-muted text-xs">No messages yet -- say hi.</p>}
        {messages.map((m, i) => (
          <div key={m.id ?? i} className={m.type === "reaction" ? "text-lg" : ""}>
            <span className="text-accent font-medium">{m.display_name}: </span>
            <span>{m.body}</span>
          </div>
        ))}
        <div ref={bottomRef} />
      </div>

      {rejection && <div className="px-3 py-1.5 text-xs text-danger border-t border-border">rejected: {rejection}</div>}

      {isLive && (
        <div className="p-2 border-t border-border">
          {!user ? (
            <p className="text-xs text-muted px-1">Sign in to chat.</p>
          ) : (
            <>
              <div className="flex gap-1 mb-2 px-1">
                {REACTIONS.map((emoji) => (
                  <button
                    key={emoji}
                    onClick={() => send("reaction", emoji)}
                    className="hover:bg-surface-2 rounded px-1 text-base"
                  >
                    {emoji}
                  </button>
                ))}
              </div>
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  send("message", input);
                  setInput("");
                }}
                className="flex gap-2"
              >
                <input
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  placeholder="Say something..."
                  className="flex-1 bg-surface-2 border border-border rounded-md px-3 py-1.5 text-sm"
                />
                <button type="submit" className="bg-accent text-black text-sm font-medium rounded-md px-3">
                  Send
                </button>
              </form>
            </>
          )}
        </div>
      )}
    </div>
  );
}
