"use client";

import Link from "next/link";
import { useAuth } from "@/lib/auth-context";
import { NotificationBell } from "./NotificationBell";

export function NavBar() {
  const { user, loading, logout } = useAuth();

  return (
    <header className="border-b border-border bg-surface px-6 py-3 flex items-center justify-between">
      <Link href="/" className="font-semibold text-lg text-accent">
        LiveStream
      </Link>
      <nav className="flex items-center gap-4 text-sm">
        {!loading && user && (
          <>
            <span className="text-muted">
              {user.display_name} <span className="text-xs">({user.role})</span>
            </span>
            {user.role === "creator" && (
              <Link href="/create-channel" className="text-accent hover:underline">
                Create channel
              </Link>
            )}
            <NotificationBell />
            <button onClick={logout} className="text-muted hover:text-foreground">
              Sign out
            </button>
          </>
        )}
        {!loading && !user && (
          <Link href="/login" className="text-accent hover:underline">
            Sign in
          </Link>
        )}
      </nav>
    </header>
  );
}
