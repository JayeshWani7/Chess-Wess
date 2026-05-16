import { create } from "zustand";
import { Chess } from "chess.js";
import type { Move } from "chess.js";

export type GameStatus = "pending" | "active" | "completed" | "abandoned";
export type GameResult = "checkmate" | "stalemate" | "timeout" | "resign" | "draw" | null;

export interface GameMove {
  id: string;
  gameId: string;
  playerId: string;
  moveNumber: number;
  moveSan: string;
  moveUci: string;
  fenAfter: string;
  createdAt: string;
}

export interface GameInfo {
  id: string;
  white_player_id: string | null;
  black_player_id: string | null;
  status: GameStatus;
  time_control: number;
  winner_id?: string | null;
  result?: string | null;
  active_timeline_id?: string | null;
}

export interface TimelineMove {
  uci: string;
  san: string;
  promotion?: string | null;
}

export interface TimelineNode {
  id: string;
  game_id: string;
  timeline_id: string;
  parent_node_id: string | null;
  move?: TimelineMove | null;
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

// Phase 5: Energy System Types
export interface PlayerEnergy {
  id: string;
  game_id: string;
  player_id: string;
  energy_remaining: number;
  energy_spent: number;
  created_at: string;
  updated_at: string;
}

export interface TimelineMetadata {
  id: string;
  timeline_id: string;
  game_id: string;
  locked_by_player_id?: string | null;
  is_locked: boolean;
  stability_score: number; // 0-100
  energy_cost_to_create: number;
  paradox_count: number;
  is_collapsed: boolean;
  created_at: string;
  updated_at: string;
}

interface GameState {
  // Active game
  activeGameId: string | null;
  gameInfo: GameInfo | null;
  chess: Chess;
  moves: GameMove[];
  selectedSquare: string | null;
  legalMoves: string[];
  playerColor: "w" | "b" | null;

  // Timers (seconds remaining)
  whiteTime: number;
  blackTime: number;

  // Status
  status: GameStatus;
  result: GameResult;
  winnerId: string | null;

  // Timeline state
  timelines: TimelineData[];
  nodesById: Record<string, TimelineNode>;
  nodesByTimeline: Record<string, TimelineNode[]>;
  activeTimelineId: string | null;
  activeTimelineLatestNodeId: string | null;
  selectedTimelineNodeId: string | null;

  // Phase 5: Energy System
  playerEnergy: PlayerEnergy | null;
  timelineMetadata: Record<string, TimelineMetadata>; // keyed by timeline_id

  // Actions
  setActiveGame: (gameId: string, info: GameInfo, color: "w" | "b") => void;
  loadMoves: (moves: GameMove[]) => void;
  applyMove: (move: Move, fen: string) => void;
  setTimelineData: (timelines: TimelineData[], activeTimelineId: string | null) => void;
  setActiveTimelineId: (timelineId: string | null) => void;
  selectTimelineNode: (nodeId: string | null) => void;
  syncActiveTimelineBoard: () => void;
  selectSquare: (square: string | null) => void;
  setTimers: (white: number, black: number) => void;
  setGameOver: (result: GameResult, winnerId: string | null) => void;
  setPlayerColor: (color: "w" | "b" | null) => void;
  leaveGame: () => void;
  // Phase 5 actions
  setPlayerEnergy: (energy: PlayerEnergy) => void;
  setTimelineMetadata: (metadata: TimelineMetadata[]) => void;
  updateTimelineMetadata: (timelineId: string, metadata: Partial<TimelineMetadata>) => void;
  consumeEnergy: (amount: number) => void;
  refundEnergy: (amount: number) => void;
}

function buildMovesFromTimeline(nodes: TimelineNode[]): GameMove[] {
  const moves: GameMove[] = [];
  for (const node of nodes) {
    if (!node.move) continue;
    moves.push({
      id: node.id,
      gameId: node.game_id,
      playerId: node.created_by_user,
      moveNumber: node.turn_number,
      moveSan: node.move.san,
      moveUci: node.move.uci,
      fenAfter: node.board_state,
      createdAt: node.created_at,
    });
  }
  return moves;
}

export const useGameStore = create<GameState>()((set, get) => ({
  activeGameId: null,
  gameInfo: null,
  chess: new Chess(),
  moves: [],
  selectedSquare: null,
  legalMoves: [],
  playerColor: null,
  whiteTime: 600,
  blackTime: 600,
  status: "pending",
  result: null,
  winnerId: null,
  timelines: [],
  nodesById: {},
  nodesByTimeline: {},
  activeTimelineId: null,
  activeTimelineLatestNodeId: null,
  selectedTimelineNodeId: null,
  // Phase 5 state
  playerEnergy: null,
  timelineMetadata: {},

  setActiveGame: (gameId, info, color) => {
    const chess = new Chess();
    set({
      activeGameId: gameId,
      gameInfo: info,
      chess,
      moves: [],
      selectedSquare: null,
      legalMoves: [],
      playerColor: color,
      whiteTime: info.time_control || 600,
      blackTime: info.time_control || 600,
      status: info.status,
      result: null,
      winnerId: null,
      timelines: [],
      nodesById: {},
      nodesByTimeline: {},
      activeTimelineId: info.active_timeline_id ?? null,
      activeTimelineLatestNodeId: null,
      selectedTimelineNodeId: null,
      playerEnergy: null,
      timelineMetadata: {},
    });
  },

  loadMoves: (moves) => {
    const chess = new Chess();
    for (const m of moves) {
      chess.move(m.moveSan);
    }
    set({ chess, moves });
  },

  applyMove: (move, fen) => {
    const { chess } = get();
    const applied = chess.move(move);
    if (!applied) return;

    // Create a new Chess instance from the resulting FEN so that
    // any useEffect depending on the `chess` reference will re-fire.
    const nextFen = fen || chess.fen();
    const newChess = new Chess(nextFen);

    const newMove: GameMove = {
      id: crypto.randomUUID(),
      gameId: get().activeGameId!,
      playerId: "",
      moveNumber: get().moves.length + 1,
      moveSan: applied.san,
      moveUci: applied.from + applied.to + (applied.promotion ?? ""),
      fenAfter: nextFen,
      createdAt: new Date().toISOString(),
    };

    set((s) => ({ chess: newChess, moves: [...s.moves, newMove] }));
  },

  setTimelineData: (timelines, activeTimelineId) => {
    const nodesById: Record<string, TimelineNode> = {};
    const nodesByTimeline: Record<string, TimelineNode[]> = {};

    for (const timeline of timelines) {
      const sorted = [...timeline.nodes].sort((a, b) => a.turn_number - b.turn_number);
      nodesByTimeline[timeline.timeline_id] = sorted;
      for (const node of sorted) {
        nodesById[node.id] = node;
      }
    }

    let resolvedActiveTimelineId = activeTimelineId ?? null;
    if (!resolvedActiveTimelineId && timelines.length > 0) {
      resolvedActiveTimelineId = timelines[0].timeline_id;
    }

    let latestNodeId: string | null = null;
    if (resolvedActiveTimelineId && nodesByTimeline[resolvedActiveTimelineId]?.length) {
      const list = nodesByTimeline[resolvedActiveTimelineId];
      latestNodeId = list[list.length - 1].id;
    }

    const moves = resolvedActiveTimelineId
      ? buildMovesFromTimeline(nodesByTimeline[resolvedActiveTimelineId] ?? [])
      : [];

    const latestFen = resolvedActiveTimelineId && nodesByTimeline[resolvedActiveTimelineId]?.length
      ? nodesByTimeline[resolvedActiveTimelineId][nodesByTimeline[resolvedActiveTimelineId].length - 1].board_state
      : null;

    set({
      timelines,
      nodesById,
      nodesByTimeline,
      activeTimelineId: resolvedActiveTimelineId,
      activeTimelineLatestNodeId: latestNodeId,
      moves,
      chess: latestFen ? new Chess(latestFen) : get().chess,
    });
  },

  setActiveTimelineId: (timelineId) => {
    const { nodesByTimeline } = get();
    if (!timelineId) {
      set({ activeTimelineId: null, activeTimelineLatestNodeId: null });
      return;
    }

    const list = nodesByTimeline[timelineId] ?? [];
    const latestNode = list.length ? list[list.length - 1] : null;
    const moves = buildMovesFromTimeline(list);

    set({
      activeTimelineId: timelineId,
      activeTimelineLatestNodeId: latestNode ? latestNode.id : null,
      moves,
      chess: latestNode ? new Chess(latestNode.board_state) : get().chess,
    });
  },

  selectTimelineNode: (nodeId) => set({ selectedTimelineNodeId: nodeId }),

  syncActiveTimelineBoard: () => {
    const { activeTimelineId, nodesByTimeline } = get();
    if (!activeTimelineId) return;
    const list = nodesByTimeline[activeTimelineId] ?? [];
    if (!list.length) return;
    const latestNode = list[list.length - 1];
    set({
      activeTimelineLatestNodeId: latestNode.id,
      moves: buildMovesFromTimeline(list),
      chess: new Chess(latestNode.board_state),
    });
  },

  selectSquare: (square) => {
    const { chess, selectedSquare, playerColor } = get();

    if (!square) {
      set({ selectedSquare: null, legalMoves: [] });
      return;
    }

    // If a square is already selected, try to make a move
    if (selectedSquare && selectedSquare !== square) {
      const piece = chess.get(selectedSquare as Parameters<typeof chess.get>[0]);
      if (piece && piece.color === playerColor) {
        const moves = chess.moves({ square: selectedSquare as Parameters<typeof chess.moves>[0]["square"], verbose: true });
        const target = moves.find((m) => m.to === square);
        if (target) {
          // Move will be handled by the component (needs promotion check)
          set({ selectedSquare: square, legalMoves: [] });
          return;
        }
      }
    }

    // Select the square and compute legal moves
    const piece = chess.get(square as Parameters<typeof chess.get>[0]);
    if (piece && piece.color === playerColor && chess.turn() === playerColor) {
      const moves = chess.moves({ square: square as Parameters<typeof chess.moves>[0]["square"], verbose: true });
      set({
        selectedSquare: square,
        legalMoves: moves.map((m) => m.to),
      });
    } else {
      set({ selectedSquare: null, legalMoves: [] });
    }
  },

  setTimers: (white, black) => set({ whiteTime: white, blackTime: black }),

  setGameOver: (result, winnerId) =>
    set({ status: "completed", result, winnerId }),

  setPlayerColor: (color) => set({ playerColor: color }),

  leaveGame: () =>
    set({
      activeGameId: null,
      gameInfo: null,
      chess: new Chess(),
      moves: [],
      selectedSquare: null,
      legalMoves: [],
      playerColor: null,
      status: "pending",
      result: null,
      winnerId: null,
      timelines: [],
      nodesById: {},
      nodesByTimeline: {},
      activeTimelineId: null,
      activeTimelineLatestNodeId: null,
      selectedTimelineNodeId: null,
      playerEnergy: null,
      timelineMetadata: {},
    }),

  // Phase 5: Energy Management Actions
  setPlayerEnergy: (energy) => set({ playerEnergy: energy }),

  setTimelineMetadata: (metadata) => {
    const timelineMetadata: Record<string, TimelineMetadata> = {};
    for (const m of metadata) {
      timelineMetadata[m.timeline_id] = m;
    }
    set({ timelineMetadata });
  },

  updateTimelineMetadata: (timelineId, metadata) => {
    set((state) => ({
      timelineMetadata: {
        ...state.timelineMetadata,
        [timelineId]: {
          ...state.timelineMetadata[timelineId],
          ...metadata,
        },
      },
    }));
  },

  consumeEnergy: (amount) => {
    set((state) => {
      if (!state.playerEnergy) return {};
      return {
        playerEnergy: {
          ...state.playerEnergy,
          energy_remaining: state.playerEnergy.energy_remaining - amount,
          energy_spent: state.playerEnergy.energy_spent + amount,
        },
      };
    });
  },

  refundEnergy: (amount) => {
    set((state) => {
      if (!state.playerEnergy) return {};
      return {
        playerEnergy: {
          ...state.playerEnergy,
          energy_remaining: state.playerEnergy.energy_remaining + amount,
        },
      };
    });
  },
}));
