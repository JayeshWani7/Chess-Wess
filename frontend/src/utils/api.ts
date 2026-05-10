const BASE = import.meta.env.VITE_API_URL ?? "";

async function request<T>(
  path: string,
  options: RequestInit = {},
  token?: string | null
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string>),
  };
  if (token) headers["Authorization"] = `Bearer ${token}`;

  const res = await fetch(`${BASE}${path}`, { ...options, headers });
  const data = await res.json();

  if (!res.ok) {
    throw new Error((data as { error?: string }).error ?? "Request failed");
  }
  return data as T;
}

export interface AuthResponse {
  token: string;
  user_id: string;
  username: string;
}

export interface GameInfo {
  id: string;
  white_player_id: string | null;
  black_player_id: string | null;
  status: string;
  time_control: number;
  winner_id?: string | null;
  result?: string | null;
  created_at?: string;
  updated_at?: string;
}

export interface GameHistoryEntry extends GameInfo {
  white_username: string;
  black_username: string;
}

export interface GameMove {
  id: string;
  game_id: string;
  player_id: string;
  move_number: number;
  move_san: string;
  move_uci: string;
  fen_after: string;
  created_at: string;
}

export interface BotInfo {
  id: string;
  username: string;
  rating: number;
}

export interface UserInfo {
  id: string;
  username: string;
  is_bot: boolean;
  rating: number;
}

export const api = {
  register: (username: string, password: string) =>
    request<AuthResponse>("/api/auth/register", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),

  login: (username: string, password: string) =>
    request<AuthResponse>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),

  listGames: (token: string) =>
    request<GameInfo[]>("/api/games", {}, token),

  createGame: (token: string, timeControl: number, color: string) =>
    request<GameInfo>(
      "/api/games",
      { method: "POST", body: JSON.stringify({ time_control: timeControl, color }) },
      token
    ),

  getGame: (token: string, gameId: string) =>
    request<GameInfo>(`/api/games/${gameId}`, {}, token),

  joinGame: (token: string, gameId: string) =>
    request<{ status: string; game_id: string }>(
      `/api/games/${gameId}/join`,
      { method: "POST" },
      token
    ),

  getGameMoves: (token: string, gameId: string) =>
    request<GameMove[]>(`/api/games/${gameId}/moves`, {}, token),

  resignGame: (token: string, gameId: string) =>
    request<{ status: string }>(
      `/api/games/${gameId}/resign`,
      { method: "POST" },
      token
    ),

  createBotGame: (
    token: string,
    timeControl: number,
    botRating: number,
    color: "white" | "black"
  ) =>
    request<GameInfo>(
      "/api/games/bot",
      {
        method: "POST",
        body: JSON.stringify({ time_control: timeControl, bot_rating: botRating, color }),
      },
      token
    ),

  listBots: (token: string) =>
    request<BotInfo[]>("/api/bots", {}, token),

  getUser: (token: string, userId: string) =>
    request<UserInfo>(`/api/users/${userId}`, {}, token),

  listMyGames: (token: string) =>
    request<GameHistoryEntry[]>("/api/games/history", {}, token),
};
