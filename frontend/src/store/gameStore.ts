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

  // Actions
  setActiveGame: (gameId: string, info: GameInfo, color: "w" | "b") => void;
  loadMoves: (moves: GameMove[]) => void;
  applyMove: (move: Move, fen: string) => void;
  selectSquare: (square: string | null) => void;
  setTimers: (white: number, black: number) => void;
  setGameOver: (result: GameResult, winnerId: string | null) => void;
  leaveGame: () => void;
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
    }),
}));
