"use client";

import { useRef, useState } from "react";
import { ApiError, uploadRecording } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import type { Stream } from "@/lib/types";

export function PastStreamRow({ stream, isOwner, onUploaded }: { stream: Stream; isOwner: boolean; onUploaded: () => void }) {
  const { token } = useAuth();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);

  async function handleFile(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file || !token) return;
    setBusy(true);
    setError(null);
    try {
      await uploadRecording(token, stream.id, file);
      onUploaded();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "upload failed");
    } finally {
      setBusy(false);
      if (fileRef.current) fileRef.current.value = "";
    }
  }

  return (
    <div className="border-b border-border pb-3">
      <div className="text-sm text-foreground">{stream.title}</div>

      {stream.recording_url && (
        <video controls src={stream.recording_url} className="mt-2 w-full max-w-md rounded-md bg-black" />
      )}

      {!stream.recording_url && isOwner && (
        <div className="mt-2 flex items-center gap-2">
          <input ref={fileRef} type="file" accept="video/*" onChange={handleFile} disabled={busy} className="text-xs text-muted" />
          {busy && <span className="text-xs text-muted">uploading...</span>}
        </div>
      )}
      {error && <p className="text-xs text-danger mt-1">{error}</p>}
    </div>
  );
}
