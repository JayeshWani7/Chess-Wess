import { create } from "zustand";
import { Chess } from "chess.js";
import type { Move } from "chess.js";
import { wsClient } from "../utils/wsClient";
import {
  LRUTracker,
  applyEviction,
  buildSummaries,
  MAX_HOT_TIMELINES,
  type TimelineSummary,
} from "./timelineMemory";

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
  stability_score: number;
  energy_cost_to_create: number;
  paradox_count: number;
  is_collapsed: boolean;
  created_at: string;
  updated_at: string;
}

interface GameState {
  activeGameId: string | null;
  gameInfo: GameInfo | null;
  chess: Chess;
  moves: GameMove[];
  selectedSquare: string | null;
  legalMoves: string[];
  playerColor: "w" | "b" | null;

  whiteTime: number;
  blackTime: number;

  status: GameStatus;
  result: GameResult;
  winnerId: string | null;

  timelines: TimelineData[];
  nodesById: Record<string, TimelineNode>;
  nodesByTimeline: Record<string, TimelineNode[]>;
  activeTimelineId: string | null;
  activeTimelineLatestNodeId: string | null;
  selectedTimelineNodeId: string | null;

  /** Lightweight summaries for ALL timelines — always in memory. */
  timelineSummaries: TimelineSummary[];

  playerEnergy: PlayerEnergy | null;
  opponentEnergy: PlayerEnergy | null;
  timelineMetadata: Record<string, TimelineMetadata>;

  setActiveGame: (gameId: string, info: GameInfo, color: "w" | "b") => void;
  loadMoves: (moves: GameMove[]) => void;
  applyMove: (move: Move, fen: string) => void;
  setTimelineData: (
    timelines: TimelineData[],
    activeTimelineId: string | null,
    merges?: { id: string; game_id: string; source_node_id: string; target_node_id: string }[],
    annotations?: { id: string; node_id: string; user_id: string; username: string; annotation: string; label_tag: string | null; created_at: string }[]
  ) => void;
  addTimelineNode: (node: TimelineNode) => void;
  addTimeline: (timelineId: string, timelineName: string | null | undefined, rootNode: TimelineNode) => void;
  renameTimelineLocal: (timelineId: string, name: string) => void;
  setActiveTimelineId: (timelineId: string | null) => void;
  selectTimelineNode: (nodeId: string | null) => void;
  syncActiveTimelineBoard: () => void;
  /** Notify the LRU that a timeline was accessed (loads full nodes if cold). */
  touchTimeline: (timelineId: string) => void;
  selectSquare: (square: string | null) => void;
  setTimers: (white: number, black: number) => void;
  setGameOver: (result: GameResult, winnerId: string | null) => void;
  setPlayerColor: (color: "w" | "b" | null) => void;
  leaveGame: () => void;
  setPlayerEnergy: (energy: PlayerEnergy) => void;
  setOpponentEnergy: (energy: PlayerEnergy | null) => void;
  setTimelineMetadata: (metadata: TimelineMetadata[]) => void;
  updateTimelineMetadata: (timelineId: string, metadata: Partial<TimelineMetadata>) => void;
  consumeEnergy: (amount: number) => void;
  refundEnergy: (amount: number) => void;

  // Merge state
  merges: { id: string; game_id: string; source_node_id: string; target_node_id: string }[];
  addMergeLocal: (sourceNodeId: string, targetNodeId: string) => void;

  // Sandbox state
  sandboxMode: boolean;
  sandboxMoves: TimelineNode[];
  sandboxParentNodeId: string | null;
  manifestQueue: { uci: string; san: string; fen: string }[];
  toggleSandboxMode: (enabled: boolean) => void;
  addSandboxMove: (move: { from: string; to: string; promotion?: string; uci: string; san: string; fen: string }) => void;
  clearSandbox: () => void;
  manifestSandbox: () => void;
  manifestNextQueueItem: (receivedNodeId: string, receivedTimelineId: string) => void;

  // Phase 4 state
  annotations: Record<string, { user_id: string; username: string; annotation: string; label_tag: string | null }[]>;
  selectedCompareNodeId: string | null;
  addAnnotationLocal: (nodeId: string, userId: string, username: string, annotation: string, labelTag: string | null) => void;
  selectCompareNode: (nodeId: string | null) => void;
}

// Module-level LRU tracker — one per browser session, reset on leaveGame.
const lru = new LRUTracker(MAX_HOT_TIMELINES);

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

function getSandboxPath(nodeId: string, nodesById: Record<string, TimelineNode>): TimelineNode[] {
  const path: TimelineNode[] = [];
  let currId: string | null = nodeId;
  while (currId && nodesById[currId]) {
    const node = nodesById[currId] as TimelineNode;
    path.push(node);
    currId = node.parent_node_id;
  }
  return path.reverse();
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
  timelineSummaries: [],
  playerEnergy: null,
  opponentEnergy: null,
  timelineMetadata: {},

  merges: [],
  sandboxMode: false,
  sandboxMoves: [],
  sandboxParentNodeId: null,
  manifestQueue: [],

  annotations: {},
  selectedCompareNodeId: null,

  setActiveGame: (gameId, info, color) => {
    lru.reset();
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
      timelineSummaries: [],
      playerEnergy: null,
      opponentEnergy: null,
      timelineMetadata: {},
      merges: [],
      sandboxMode: false,
      sandboxMoves: [],
      sandboxParentNodeId: null,
      manifestQueue: [],
      annotations: {},
      selectedCompareNodeId: null,
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

  setTimelineData: (timelines, activeTimelineId, merges, annotations) => {
    // Always mark active timeline as hot
    if (activeTimelineId) lru.pin(activeTimelineId);
    const hotSet = lru.hotSet();
    const pinnedIds = new Set(activeTimelineId ? [activeTimelineId] : []);
    const evicted = applyEviction(timelines, hotSet, pinnedIds);

    const nodesById: Record<string, TimelineNode> = {};
    const nodesByTimeline: Record<string, TimelineNode[]> = {};

    for (const timeline of evicted) {
      const sorted = [...timeline.nodes].sort((a, b) => a.turn_number - b.turn_number);
      nodesByTimeline[timeline.timeline_id] = sorted;
      for (const node of sorted) {
        nodesById[node.id] = node;
      }
    }

    let resolvedActiveTimelineId = activeTimelineId ?? null;
    if (!resolvedActiveTimelineId && evicted.length > 0) {
      resolvedActiveTimelineId = evicted[0].timeline_id;
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

    const annotationsMap: Record<string, { user_id: string; username: string; annotation: string; label_tag: string | null }[]> = {};
    if (annotations) {
      for (const a of annotations) {
        if (!annotationsMap[a.node_id]) {
          annotationsMap[a.node_id] = [];
        }
        annotationsMap[a.node_id].push({
          user_id: a.user_id,
          username: a.username,
          annotation: a.annotation,
          label_tag: a.label_tag,
        });
      }
    }

    set({
      timelines: evicted,
      nodesById,
      nodesByTimeline,
      activeTimelineId: resolvedActiveTimelineId,
      activeTimelineLatestNodeId: latestNodeId,
      timelineSummaries: buildSummaries(timelines), // summaries always from full data
      moves,
      chess: latestFen ? new Chess(latestFen) : get().chess,
      merges: merges ?? get().merges,
      annotations: annotations ? annotationsMap : get().annotations,
    });
  },

  addTimelineNode: (node) => {
    set((state) => {
      if (state.nodesById[node.id]) {
        return {};
      }

      const nodesById = { ...state.nodesById, [node.id]: node };
      const nodesByTimeline = { ...state.nodesByTimeline };
      const existing = nodesByTimeline[node.timeline_id] ?? [];
      const updatedNodes = [...existing, node].sort((a, b) => a.turn_number - b.turn_number);
      nodesByTimeline[node.timeline_id] = updatedNodes;

      let found = false;
      const timelines = state.timelines.map((timeline) => {
        if (timeline.timeline_id !== node.timeline_id) return timeline;
        found = true;
        return {
          ...timeline,
          nodes: updatedNodes,
          node_count: (timeline.node_count ?? existing.length) + 1,
        };
      });

      if (!found) {
        timelines.push({
          timeline_id: node.timeline_id,
          nodes: updatedNodes,
          node_count: updatedNodes.length,
          nodes_partial: false,
        });
      }

      const nextState: Partial<GameState> = {
        nodesById,
        nodesByTimeline,
        timelines,
        timelineSummaries: buildSummaries(timelines),
      };

      if (state.activeTimelineId === node.timeline_id) {
        const latestNode = updatedNodes[updatedNodes.length - 1];
        nextState.activeTimelineLatestNodeId = latestNode.id;
        nextState.moves = buildMovesFromTimeline(updatedNodes);
        nextState.chess = new Chess(latestNode.board_state);
      }

      return nextState;
    });
  },

  addTimeline: (timelineId, timelineName, rootNode) => {
    set((state) => {
      if (state.timelines.some((timeline) => timeline.timeline_id === timelineId)) {
        return {};
      }

      const nodesById = { ...state.nodesById, [rootNode.id]: rootNode };
      const nodesByTimeline = { ...state.nodesByTimeline, [timelineId]: [rootNode] };
      const timelines = [
        ...state.timelines,
        {
          timeline_id: timelineId,
          timeline_name: timelineName ?? null,
          nodes: [rootNode],
          node_count: 1,
          nodes_partial: false,
        },
      ];

      return { nodesById, nodesByTimeline, timelines, timelineSummaries: buildSummaries(timelines) };
    });
  },

  renameTimelineLocal: (timelineId, name) => {
    set((state) => ({
      timelines: state.timelines.map((timeline) =>
        timeline.timeline_id === timelineId
          ? { ...timeline, timeline_name: name }
          : timeline
      ),
    }));
  },

  setActiveTimelineId: (timelineId) => {
    const { nodesByTimeline } = get();
    if (!timelineId) {
      set({ activeTimelineId: null, activeTimelineLatestNodeId: null });
      return;
    }

    // Mark hot in LRU — evict a cold timeline if over budget
    const evicted = lru.touch(timelineId);
    if (evicted) {
      // Demote evicted timeline to stub in the store
      set((state) => ({
        timelines: state.timelines.map((t) =>
          t.timeline_id === evicted
            ? { ...t, nodes: t.nodes.slice(-1), nodes_partial: true }
            : t
        ),
        nodesByTimeline: (() => {
          const updated = { ...state.nodesByTimeline };
          const stub = updated[evicted]?.slice(-1) ?? [];
          updated[evicted] = stub;
          return updated;
        })(),
      }));
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

  touchTimeline: (timelineId) => {
    // Record access in LRU; if the returned evicted id is set, demote that timeline
    const evicted = lru.touch(timelineId);
    if (evicted) {
      set((state) => {
        const stub = (state.nodesByTimeline[evicted] ?? []).slice(-1);
        return {
          timelines: state.timelines.map((t) =>
            t.timeline_id === evicted
              ? { ...t, nodes: stub, nodes_partial: true }
              : t
          ),
          nodesByTimeline: { ...state.nodesByTimeline, [evicted]: stub },
        };
      });
    }
  },

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
    const { chess, selectedSquare, playerColor, sandboxMode } = get();

    if (!square) {
      set({ selectedSquare: null, legalMoves: [] });
      return;
    }

    const activeColor = sandboxMode ? chess.turn() : playerColor;

    if (selectedSquare && selectedSquare !== square) {
      const piece = chess.get(selectedSquare as Parameters<typeof chess.get>[0]);
      if (piece && piece.color === activeColor) {
        const moves = chess.moves({ square: selectedSquare as Parameters<typeof chess.moves>[0]["square"], verbose: true });
        const target = moves.find((m) => m.to === square);
        if (target) {
          set({ selectedSquare: square, legalMoves: [] });
          return;
        }
      }
    }

    const piece = chess.get(square as Parameters<typeof chess.get>[0]);
    if (piece && piece.color === activeColor && chess.turn() === activeColor) {
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

  leaveGame: () => {
    lru.reset();
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
      timelineSummaries: [],
      playerEnergy: null,
      opponentEnergy: null,
      timelineMetadata: {},
      merges: [],
      sandboxMode: false,
      sandboxMoves: [],
      sandboxParentNodeId: null,
      manifestQueue: [],
      annotations: {},
      selectedCompareNodeId: null,
    });
  },

  setPlayerEnergy: (energy) => set({ playerEnergy: energy }),

  setOpponentEnergy: (energy) => set({ opponentEnergy: energy }),

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

  addMergeLocal: (source, target) => {
    set((state) => ({
      merges: [
        ...state.merges,
        {
          id: `local-merge-${Date.now()}`,
          game_id: state.activeGameId!,
          source_node_id: source,
          target_node_id: target,
        },
      ],
    }));
  },

  toggleSandboxMode: (enabled) => {
    const { selectedTimelineNodeId, activeTimelineLatestNodeId } = get();
    if (enabled) {
      const parentId = selectedTimelineNodeId || activeTimelineLatestNodeId;
      set({
        sandboxMode: true,
        sandboxMoves: [],
        sandboxParentNodeId: parentId,
        manifestQueue: [],
      });
    } else {
      get().clearSandbox();
    }
  },

  addSandboxMove: (move) => {
    const { sandboxMoves, sandboxParentNodeId, nodesById, activeGameId } = get();
    const parentId = sandboxMoves.length === 0 ? sandboxParentNodeId : sandboxMoves[sandboxMoves.length - 1].id;
    const parentNode = parentId ? nodesById[parentId] : null;
    const turnNumber = (parentNode ? parentNode.turn_number : 0) + 1;
    
    const newSandboxNode: TimelineNode = {
      id: `sandbox-${crypto.randomUUID()}`,
      game_id: activeGameId!,
      timeline_id: "sandbox",
      parent_node_id: parentId,
      move: { uci: move.uci, san: move.san, promotion: move.promotion },
      board_state: move.fen,
      turn_number: turnNumber,
      created_by_user: "sandbox-user",
      created_at: new Date().toISOString(),
    };

    const newMoves = [...sandboxMoves, newSandboxNode];
    const newChess = new Chess(move.fen);
    
    // Create local GameMove list representing the path
    const pathNodes = [...(parentNode && parentId ? getSandboxPath(parentId, nodesById) : []), newSandboxNode];
    const newGameMoves = buildMovesFromTimeline(pathNodes);

    set((s) => ({
      sandboxMoves: newMoves,
      chess: newChess,
      moves: newGameMoves,
      selectedTimelineNodeId: newSandboxNode.id,
      nodesById: { ...s.nodesById, [newSandboxNode.id]: newSandboxNode },
    }));
  },

  clearSandbox: () => {
    const { sandboxParentNodeId, nodesById, activeTimelineLatestNodeId } = get();
    set({
      sandboxMode: false,
      sandboxMoves: [],
      sandboxParentNodeId: null,
      manifestQueue: [],
    });
    
    // Restore chessboard state to the previous node
    const targetNodeId = sandboxParentNodeId || activeTimelineLatestNodeId;
    if (targetNodeId && nodesById[targetNodeId]) {
      const node = nodesById[targetNodeId];
      const pathNodes = getSandboxPath(targetNodeId, nodesById);
      set({
        chess: new Chess(node.board_state),
        moves: buildMovesFromTimeline(pathNodes),
        selectedTimelineNodeId: targetNodeId,
      });
    }
  },

  manifestSandbox: () => {
    const { sandboxMoves, sandboxParentNodeId } = get();
    if (sandboxMoves.length === 0) return;

    // Convert sandbox moves to the manifestQueue
    const queue = sandboxMoves.map((m) => ({
      uci: m.move!.uci,
      san: m.move!.san,
      fen: m.board_state,
    }));

    // Start manifestation by popping and sending the first move
    const first = queue[0];
    const remaining = queue.slice(1);

    set({
      manifestQueue: remaining,
      sandboxMode: false,
      sandboxMoves: [],
      sandboxParentNodeId: null,
    });

    // Send first move using the parentNodeId we started from
    wsClient.sendMove(
      first.uci,
      first.san,
      first.fen,
      null, // let server resolve or match
      sandboxParentNodeId
    );
  },

  manifestNextQueueItem: (receivedNodeId, receivedTimelineId) => {
    const { manifestQueue } = get();
    if (manifestQueue.length === 0) return;

    const first = manifestQueue[0];
    const remaining = manifestQueue.slice(1);

    set({ manifestQueue: remaining });

    // Send the next move in the chain, referencing the parent we just received
    wsClient.sendMove(
      first.uci,
      first.san,
      first.fen,
      receivedTimelineId,
      receivedNodeId
    );
  },

  addAnnotationLocal: (nodeId, userId, username, annotation, labelTag) => {
    set((state) => {
      const list = state.annotations[nodeId] ?? [];
      const filtered = list.filter((a) => a.user_id !== userId);
      const updated = [
        ...filtered,
        {
          user_id: userId,
          username: username,
          annotation: annotation,
          label_tag: labelTag,
        },
      ];
      return {
        annotations: {
          ...state.annotations,
          [nodeId]: updated,
        },
      };
    });
  },

  selectCompareNode: (nodeId) => {
    set({ selectedCompareNodeId: nodeId });
  },
}));
