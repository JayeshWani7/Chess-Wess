import { useAuthStore } from "../store/authStore";

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

  if (res.status === 401) {
    useAuthStore.getState().logout();
  }

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
  active_timeline_id?: string | null;
  winner_id?: string | null;
  result?: string | null;
  created_at?: string;
  updated_at?: string;
}

export interface GameHistoryEntry extends GameInfo {
  white_username: string;
  black_username: string;
}

export interface GameHistoryPage {
  games: GameHistoryEntry[];
  total: number;
  page: number;
  limit: number;
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

export interface TimelineNode {
  id: string;
  game_id: string;
  timeline_id: string;
  parent_node_id: string | null;
  move?: { uci: string; san: string; promotion?: string | null } | null;
  board_state: string;
  turn_number: number;
  created_by_user: string;
  created_at: string;
  metadata?: {
    check: boolean;
    checkmate: boolean;
    stalemate: boolean;
    evaluation?: number | null;
    captured?: string | null;
  };
}

export interface TimelineData {
  timeline_id: string;
  timeline_name?: string | null;
  nodes: TimelineNode[];
  node_count?: number;
  nodes_partial?: boolean;
}

export interface NodeAnnotation {
  id: string;
  node_id: string;
  user_id: string;
  username: string;
  annotation: string;
  label_tag: string | null;
  created_at: string;
}

export interface GameTimelineResponse {
  game_id: string;
  active_timeline_id: string | null;
  timelines: TimelineData[];
  annotations?: NodeAnnotation[];
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

  getGameTimeline: (token: string, gameId: string, nodeLimit?: number | null) => {
    const query = nodeLimit && nodeLimit > 0 ? `?node_limit=${nodeLimit}` : "";
    return request<GameTimelineResponse>(`/api/games/${gameId}/timeline${query}`, {}, token);
  },

  getActiveTimeline: (token: string, gameId: string) =>
    request<{ game_id: string; active_timeline_id: string | null }>(
      `/api/games/${gameId}/timeline/active`,
      {},
      token
    ),

  setActiveTimeline: (token: string, gameId: string, timelineId: string) =>
    request<{ game_id: string; active_timeline_id: string }>(
      `/api/games/${gameId}/timeline/active`,
      { method: "POST", body: JSON.stringify({ timeline_id: timelineId }) },
      token
    ),

  renameTimeline: (token: string, gameId: string, timelineId: string, timelineName: string) =>
    request<{ game_id: string; timeline_id: string; timeline_name: string }>(
      `/api/games/${gameId}/timeline`,
      {
        method: "POST",
        body: JSON.stringify({ timeline_id: timelineId, timeline_name: timelineName }),
      },
      token
    ),

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

  listMyGames: (
    token: string,
    page = 1,
    limit = 10,
    filter: "all" | "win" | "loss" | "draw" = "all"
  ) =>
    request<GameHistoryPage>(
      `/api/games/history?page=${page}&limit=${limit}&filter=${filter}`,
      {},
      token
    ),
  getPlayerEnergy: (token: string, gameId: string) =>
    request<any>(`/api/games/${gameId}/energy`, {}, token),

  getOpponentEnergy: (token: string, gameId: string, opponentId: string) =>
    request<any>(`/api/games/${gameId}/energy/${opponentId}`, {}, token),

  spendEnergy: (
    token: string,
    gameId: string,
    amount: number,
    action: "rewind" | "jump_timeline" | "lock" | "paradox_penalty",
    details: string
  ) =>
    request<any>(
      `/api/games/${gameId}/energy/spend`,
      {
        method: "POST",
        body: JSON.stringify({ amount, action, details }),
      },
      token
    ),

  refundEnergy: (token: string, gameId: string, amount: number, reason: string) =>
    request<any>(
      `/api/games/${gameId}/energy/refund`,
      {
        method: "POST",
        body: JSON.stringify({ amount, reason }),
      },
      token
    ),

  lockTimeline: (token: string, gameId: string, timelineId: string) =>
    request<any>(
      `/api/games/${gameId}/energy/lock-timeline`,
      {
        method: "POST",
        body: JSON.stringify({ timeline_id: timelineId }),
      },
      token
    ),

  getTimelineStatus: (token: string, gameId: string, timelineId: string) =>
    request<any>(
      `/api/games/${gameId}/energy/timeline-status?timeline_id=${timelineId}`,
      {},
      token
    ),

  getEnergyStatus: (token: string, gameId: string) =>
    request<any>(`/api/games/${gameId}/energy/status`, {}, token),

  mergeTimelines: (token: string, gameId: string, sourceNodeId: string, targetNodeId: string) =>
    request<{ game_id: string; source_node_id: string; target_node_id: string }>(
      `/api/games/${gameId}/merge`,
      {
        method: "POST",
        body: JSON.stringify({ source_node_id: sourceNodeId, target_node_id: targetNodeId }),
      },
      token
    ),

  annotateNode: (token: string, gameId: string, nodeId: string, annotation: string, labelTag: string) =>
    request<{ status: string }>(
      `/api/games/${gameId}/annotation`,
      {
        method: "POST",
        body: JSON.stringify({ node_id: nodeId, annotation, label_tag: labelTag }),
      },
      token
    ),
};
