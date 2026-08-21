export type Role = "viewer" | "creator";

export interface User {
  id: string;
  email: string;
  display_name: string;
  role: Role;
  created_at: string;
}

export interface AuthResponse {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  user: User;
}

export interface Channel {
  id: string;
  creator_id: string;
  slug: string;
  name: string;
  description: string;
  category: string;
  created_at: string;
}

export type StreamStatus = "scheduled" | "live" | "ended";

export interface Stream {
  id: string;
  channel_id: string;
  title: string;
  tags: string[];
  status: StreamStatus;
  scheduled_start_at: string;
  started_at?: string;
  ended_at?: string;
  created_at: string;
  viewer_count: number;
  recording_url?: string;
}

export type ChatMessageType = "message" | "reaction";

export interface ChatMessage {
  id: string;
  stream_id: string;
  user_id: string;
  display_name: string;
  type: ChatMessageType;
  body: string;
  deleted_at?: string;
  created_at: string;
}
