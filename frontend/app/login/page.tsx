"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { ApiError, login, signup } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";

export default function LoginPage() {
  const router = useRouter();
  const { setSession } = useAuth();

  const [mode, setMode] = useState<"login" | "signup">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [role, setRole] = useState<"viewer" | "creator">("viewer");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      const auth =
        mode === "login" ? await login(email, password) : await signup(email, password, displayName, role);
      setSession(auth);
      router.push("/");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "something went wrong");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="max-w-sm mx-auto mt-16 px-4">
      <h1 className="text-xl font-semibold mb-6">{mode === "login" ? "Sign in" : "Create an account"}</h1>
      <form onSubmit={submit} className="flex flex-col gap-3">
        <input
          type="email"
          placeholder="Email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
          className="bg-surface border border-border rounded-md px-3 py-2 text-sm"
        />
        <input
          type="password"
          placeholder="Password (8+ characters)"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
          minLength={8}
          className="bg-surface border border-border rounded-md px-3 py-2 text-sm"
        />
        {mode === "signup" && (
          <>
            <input
              type="text"
              placeholder="Display name"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              required
              className="bg-surface border border-border rounded-md px-3 py-2 text-sm"
            />
            <div className="flex gap-4 text-sm text-muted">
              <label className="flex items-center gap-1.5">
                <input type="radio" checked={role === "viewer"} onChange={() => setRole("viewer")} />
                Viewer
              </label>
              <label className="flex items-center gap-1.5">
                <input type="radio" checked={role === "creator"} onChange={() => setRole("creator")} />
                Creator
              </label>
            </div>
          </>
        )}

        {error && <p className="text-danger text-sm">{error}</p>}

        <button
          type="submit"
          disabled={busy}
          className="bg-accent text-black font-medium rounded-md px-3 py-2 text-sm mt-2 disabled:opacity-50"
        >
          {busy ? "..." : mode === "login" ? "Sign in" : "Create account"}
        </button>
      </form>

      <button
        onClick={() => setMode(mode === "login" ? "signup" : "login")}
        className="text-muted text-sm mt-4 hover:text-foreground"
      >
        {mode === "login" ? "Need an account? Sign up" : "Already have an account? Sign in"}
      </button>
    </div>
  );
}
