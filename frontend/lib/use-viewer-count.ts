"use client";

import { useEffect, useState } from "react";
import { signalingUrl } from "./api";

// Connects to stream-service's WebRTC signaling socket purely to receive
// viewer-count broadcasts -- the same mechanism the /demo page in
// stream-service uses, just consumed by the real frontend instead of a
// debug page. This does not do any WebRTC negotiation; a real "watch"
// experience would additionally drive an RTCPeerConnection off the same
// socket's offer/answer/ICE messages once a broadcaster is live.
export function useViewerCount(streamId: string | null): number {
  const [count, setCount] = useState(0);

  useEffect(() => {
    if (!streamId) return;
    const ws = new WebSocket(signalingUrl(streamId));
    ws.onmessage = (evt) => {
      const msg = JSON.parse(evt.data);
      if (msg.type === "viewer-count") setCount(msg.payload.count);
    };
    return () => ws.close();
  }, [streamId]);

  return count;
}
