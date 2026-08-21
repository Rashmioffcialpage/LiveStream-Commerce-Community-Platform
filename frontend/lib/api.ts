import type { AuthResponse, Channel, ChatMessage, Stream, Subscription, User } from "./types";

const AUTH_URL = process.env.NEXT_PUBLIC_AUTH_URL!;
const STREAM_URL = process.env.NEXT_PUBLIC_STREAM_URL!;
const CHAT_URL = process.env.NEXT_PUBLIC_CHAT_URL!;
const SUBSCRIPTION_URL = process.env.NEXT_PUBLIC_SUBSCRIPTION_URL!;
const COMMERCE_URL = process.env.NEXT_PUBLIC_COMMERCE_URL!;

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, {
    ...init,
    headers: { "content-type": "application/json", ...init?.headers },
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new ApiError(res.status, body.error ?? res.statusText);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

function authHeader(token: string) {
  return { Authorization: `Bearer ${token}` };
}

// --- auth-service ---

export function signup(
  email: string,
  password: string,
  displayName: string,
  role: "viewer" | "creator",
): Promise<AuthResponse> {
  return request(`${AUTH_URL}/signup`, {
    method: "POST",
    body: JSON.stringify({ email, password, display_name: displayName, role }),
  });
}

export function login(email: string, password: string): Promise<AuthResponse> {
  return request(`${AUTH_URL}/login`, {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
}

export function me(token: string): Promise<User> {
  return request(`${AUTH_URL}/me`, { headers: authHeader(token) });
}

// --- stream-service ---

export function listChannels(): Promise<Channel[]> {
  return request(`${STREAM_URL}/channels`);
}

export function getChannel(slug: string): Promise<Channel> {
  return request(`${STREAM_URL}/channels/${slug}`);
}

export function createChannel(
  token: string,
  slug: string,
  name: string,
  description: string,
  category: string,
): Promise<Channel> {
  return request(`${STREAM_URL}/channels`, {
    method: "POST",
    headers: authHeader(token),
    body: JSON.stringify({ slug, name, description, category }),
  });
}

export function listChannelStreams(slug: string): Promise<Stream[]> {
  return request(`${STREAM_URL}/channels/${slug}/streams`);
}

export function getStream(id: string): Promise<Stream> {
  return request(`${STREAM_URL}/streams/${id}`);
}

export function createStream(
  token: string,
  slug: string,
  title: string,
  tags: string[],
): Promise<Stream> {
  return request(`${STREAM_URL}/channels/${slug}/streams`, {
    method: "POST",
    headers: authHeader(token),
    body: JSON.stringify({ title, tags, scheduled_start_at: new Date().toISOString() }),
  });
}

export function goLive(token: string, streamId: string): Promise<Stream> {
  return request(`${STREAM_URL}/streams/${streamId}/go-live`, {
    method: "POST",
    headers: authHeader(token),
  });
}

export function endStream(token: string, streamId: string): Promise<Stream> {
  return request(`${STREAM_URL}/streams/${streamId}/end`, {
    method: "POST",
    headers: authHeader(token),
  });
}

export function signalingUrl(streamId: string): string {
  return `${STREAM_URL.replace("http", "ws")}/streams/${streamId}/signal?role=viewer`;
}

export async function uploadRecording(token: string, streamId: string, file: File): Promise<Stream> {
  const form = new FormData();
  form.append("file", file);
  const res = await fetch(`${STREAM_URL}/streams/${streamId}/recording`, {
    method: "POST",
    headers: authHeader(token), // no content-type -- fetch sets the multipart boundary itself
    body: form,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new ApiError(res.status, body.error ?? res.statusText);
  }
  return res.json();
}

// --- chat-service ---

export function chatHistory(streamId: string): Promise<ChatMessage[]> {
  return request(`${CHAT_URL}/streams/${streamId}/chat/history`);
}

export function chatWsUrl(streamId: string, token: string): string {
  return `${CHAT_URL.replace("http", "ws")}/streams/${streamId}/chat?token=${encodeURIComponent(token)}`;
}

export function muteUser(
  token: string,
  streamId: string,
  userId: string,
  durationSeconds?: number,
): Promise<unknown> {
  return request(`${CHAT_URL}/streams/${streamId}/chat/mute`, {
    method: "POST",
    headers: authHeader(token),
    body: JSON.stringify({ user_id: userId, duration_seconds: durationSeconds }),
  });
}

export function deleteMessage(token: string, messageId: string): Promise<ChatMessage> {
  return request(`${CHAT_URL}/messages/${messageId}`, {
    method: "DELETE",
    headers: authHeader(token),
  });
}

// --- subscription-service ---

export function subscribeToChannel(token: string, slug: string): Promise<Subscription> {
  return request(`${SUBSCRIPTION_URL}/channels/${slug}/subscribe`, {
    method: "POST",
    headers: authHeader(token),
    body: JSON.stringify({}),
  });
}

export function listSubscribers(token: string, slug: string): Promise<Subscription[]> {
  return request(`${SUBSCRIPTION_URL}/channels/${slug}/subscribers`, {
    headers: authHeader(token),
  });
}

export function mySubscriptions(token: string): Promise<Subscription[]> {
  return request(`${SUBSCRIPTION_URL}/me/subscriptions`, {
    headers: authHeader(token),
  });
}

export function cancelSubscription(token: string, id: string): Promise<Subscription> {
  return request(`${SUBSCRIPTION_URL}/subscriptions/${id}/cancel`, {
    method: "POST",
    headers: authHeader(token),
  });
}

// --- commerce-service ---

export interface Wallet {
  user_id: string;
  coin_balance: number;
  updated_at: string;
}

export interface Gift {
  id: string;
  sender_id: string;
  recipient_id: string;
  channel_id: string;
  gift_type: string;
  coin_cost: number;
  created_at: string;
}

export const GIFT_CATALOG: Record<string, number> = { rose: 10, heart: 50, diamond: 500, rocket: 1000 };

export function getWallet(token: string): Promise<Wallet> {
  return request(`${COMMERCE_URL}/wallet`, { headers: authHeader(token) });
}

export function buyCoins(token: string, coins: number): Promise<Wallet> {
  return request(`${COMMERCE_URL}/wallet/buy-coins`, {
    method: "POST",
    headers: authHeader(token),
    body: JSON.stringify({ coins }),
  });
}

export function sendGift(token: string, slug: string, giftType: string): Promise<Gift> {
  return request(`${COMMERCE_URL}/channels/${slug}/gift`, {
    method: "POST",
    headers: authHeader(token),
    body: JSON.stringify({ gift_type: giftType }),
  });
}
