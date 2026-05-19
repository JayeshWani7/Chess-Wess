import { useEffect, useRef, useState } from "react";
import { Chess } from "chess.js";
import { api, type GameHistoryEntry, type GameMove } from "../utils/api";
import { useAuthStore } from "../store/authStore";

const FILES = ["a", "b", "c", "d", "e", "f", "g", "h"];
const RANKS = ["8", "7", "6", "5", "4", "3", "2", "1"];

const PIECES: Record<string, string> = {
  wk: "♔", wq: "♕", wr: "♖", wb: "♗", wn: "♘", wp: "♙",
  bk: "♚", bq: "♛", br: "♜", bb: "♝", bn: "♞", bp: "♟",
};

interface Props {
  game: GameHistoryEntry;
  onBack: () => void;
}

export default function GameReviewPage({ game, onBack }: Props) {
  const { token, userId } = useAuthStore();
  const [moves, setMoves] = useState<GameMove[]>([]);
  const [cursor, setCursor] = useState(-1);
  const [loading, setLoading] = useState(true);
  const moveListRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!token) return;
    api.getGameMoves(token, game.id).then((m) => {
      setMoves(m);
      setCursor(m.length - 1);
      setLoading(false);
    });
  }, [token, game.id]);

  useEffect(() => {
    const el = moveListRef.current?.querySelector(`[data-idx="${cursor}"]`);
    el?.scrollIntoView({ block: "nearest", behavior: "smooth" });
  }, [cursor]);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "ArrowLeft")  setCursor((c) => Math.max(-1, c - 1));
      if (e.key === "ArrowRight") setCursor((c) => Math.min(moves.length - 1, c + 1));
      if (e.key === "ArrowUp")    setCursor(-1);
      if (e.key === "ArrowDown")  setCursor(moves.length - 1);
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [moves.length]);

  const chess = new Chess();
  if (cursor >= 0 && moves.length > 0) {
    try {
      chess.load(moves[cursor].fen_after);
    } catch {
      const tmp = new Chess();
      for (let i = 0; i <= cursor; i++) tmp.move(moves[i].move_san);
      chess.load(tmp.fen());
    }
  }

  const lastMoveSquares =
    cursor >= 0
      ? [moves[cursor].move_uci.slice(0, 2), moves[cursor].move_uci.slice(2, 4)]
      : [];

  const flipped =
    game.black_player_id === userId &&
    game.white_player_id !== userId;
  const ranks = flipped ? [...RANKS].reverse() : RANKS;
  const fileOrder = flipped ? [...FILES].reverse() : FILES;

  const resultLabel = (() => {
    const r = game.result;
    if (!r) return "";
    const map: Record<string, string> = {
      checkmate: "Checkmate",
      stalemate: "Stalemate",
      resign: "Resignation",
      timeout: "Timeout",
      draw: "Draw",
    };
    return map[r] ?? r;
  })();

  const didWin = game.winner_id === userId;
  const isDraw = game.result === "stalemate" || game.result === "draw";
  const outcomeLabel = isDraw ? "Draw" : didWin ? "Victory" : "Defeat";
  const outcomeColor = isDraw
    ? "text-gray-400"
    : didWin
    ? "text-green-400"
    : "text-red-400";

  const pairs: Array<{ num: number; white: GameMove | null; black: GameMove | null; wi: number; bi: number }> = [];
  for (let i = 0; i < moves.length; i += 2) {
    pairs.push({ num: i / 2 + 1, white: moves[i] ?? null, black: moves[i + 1] ?? null, wi: i, bi: i + 1 });
  }

  return (
    <div className="min-h-screen p-4 max-w-5xl mx-auto">
      <div className="flex items-center justify-between mb-4">
        <button onClick={onBack} className="btn-ghost text-sm">← Back</button>
        <h1 className="text-xl font-bold text-chrono-accent">♟ Game Review</h1>
        <div className="text-right">
          <span className={`font-bold text-sm ${outcomeColor}`}>{outcomeLabel}</span>
          {resultLabel && <span className="text-gray-500 text-xs ml-2">· {resultLabel}</span>}
        </div>
      </div>

      <div className="flex items-center justify-between mb-4 px-1">
        <div className="flex items-center gap-2">
          <span className="text-lg">⬜</span>
          <span className="font-semibold text-sm">{game.white_username}</span>
        </div>
        <span className="text-gray-500 text-xs">vs</span>
        <div className="flex items-center gap-2">
          <span className="font-semibold text-sm">{game.black_username}</span>
          <span className="text-lg">⬛</span>
        </div>
      </div>

      <div className="flex flex-col lg:flex-row gap-6 items-start justify-center">
        <div className="flex flex-col items-center gap-2">
          <div className="relative select-none">
            <div
              className="grid border-2 border-chrono-border rounded-sm overflow-hidden shadow-2xl"
              style={{
                gridTemplateColumns: "repeat(8, 1fr)",
                gridTemplateRows: "repeat(8, 1fr)",
                width: "min(80vw, 480px)",
                height: "min(80vw, 480px)",
              }}
            >
              {ranks.map((rank) =>
                fileOrder.map((file) => {
                  const sq = `${file}${rank}`;
                  const piece = chess.get(sq as Parameters<typeof chess.get>[0]);
                  const isLight = (FILES.indexOf(file) + RANKS.indexOf(rank)) % 2 === 0;
                  const isLastMove = lastMoveSquares.includes(sq);

                  let bg = isLight ? "bg-board-light" : "bg-board-dark";
                  if (isLastMove) bg = "bg-board-lastmove";

                  return (
                    <div
                      key={sq}
                      className={`relative flex items-center justify-center ${bg}`}
                      style={{ aspectRatio: "1 / 1", minWidth: 0, minHeight: 0 }}
                    >
                      {piece && (
                        <span
                          className="text-4xl leading-none select-none absolute inset-0 flex items-center justify-center"
                          style={{
                            textShadow:
                              piece.color === "w"
                                ? "0 0 2px #000, 0 0 2px #000"
                                : "0 0 2px #fff4, 0 0 1px #0008",
                            filter:
                              piece.color === "w"
                                ? "drop-shadow(0 1px 1px rgba(0,0,0,0.6))"
                                : "none",
                          }}
                        >
                          {PIECES[`${piece.color}${piece.type}`] ?? "?"}
                        </span>
                      )}
                    </div>
                  );
                })
              )}
            </div>

            <div className="flex mt-1" style={{ width: "min(80vw, 480px)" }}>
              {fileOrder.map((f) => (
                <span key={f} className="flex-1 text-center text-xs text-gray-500">{f}</span>
              ))}
            </div>

            <div
              className="absolute top-0 left-0 flex flex-col"
              style={{ height: "min(80vw, 480px)", transform: "translateX(-16px)" }}
            >
              {ranks.map((r) => (
                <span key={r} className="flex-1 flex items-center text-xs text-gray-500">{r}</span>
              ))}
            </div>
          </div>

          <div className="flex items-center gap-2 mt-1">
            <button
              onClick={() => setCursor(-1)}
              disabled={cursor === -1}
              className="btn-ghost text-lg px-3 py-1 disabled:opacity-30"
              title="Start (↑)"
            >⏮</button>
            <button
              onClick={() => setCursor((c) => Math.max(-1, c - 1))}
              disabled={cursor === -1}
              className="btn-ghost text-lg px-3 py-1 disabled:opacity-30"
              title="Previous (←)"
            >◀</button>
            <span className="text-xs text-gray-500 w-20 text-center tabular-nums">
              {cursor === -1 ? "Start" : `Move ${cursor + 1} / ${moves.length}`}
            </span>
            <button
              onClick={() => setCursor((c) => Math.min(moves.length - 1, c + 1))}
              disabled={cursor === moves.length - 1}
              className="btn-ghost text-lg px-3 py-1 disabled:opacity-30"
              title="Next (→)"
            >▶</button>
            <button
              onClick={() => setCursor(moves.length - 1)}
              disabled={cursor === moves.length - 1}
              className="btn-ghost text-lg px-3 py-1 disabled:opacity-30"
              title="End (↓)"
            >⏭</button>
          </div>
          <p className="text-xs text-gray-600">Use ← → arrow keys to navigate</p>
        </div>

        <div className="card flex flex-col w-full lg:w-64" style={{ maxHeight: "520px" }}>
          <h3 className="text-sm font-semibold text-gray-400 mb-2 shrink-0">
            Moves · {moves.length} total
          </h3>

          {loading ? (
            <p className="text-gray-600 text-xs">Loading…</p>
          ) : (
            <div className="overflow-y-auto flex-1 pr-1" ref={moveListRef}>
              <div className="grid grid-cols-[2rem_1fr_1fr] gap-x-2 px-1 mb-1 text-xs text-gray-500 font-semibold uppercase tracking-wide shrink-0">
                <span>#</span><span>White</span><span>Black</span>
              </div>

              <div className="space-y-0.5 font-mono text-sm">
                {pairs.map(({ num, white, black, wi, bi }) => (
                  <div
                    key={num}
                    className="grid grid-cols-[2rem_1fr_1fr] gap-x-2 rounded px-1 py-0.5"
                  >
                    <span className="text-gray-500 text-right">{num}.</span>

                    {white && (
                      <button
                        data-idx={wi}
                        onClick={() => setCursor(wi)}
                        className={`rounded px-1 text-left transition-colors ${
                          cursor === wi
                            ? "bg-chrono-accent/40 text-white font-semibold"
                            : "text-gray-200 hover:bg-white/10"
                        }`}
                      >
                        {white.move_san}
                      </button>
                    )}

                    {black ? (
                      <button
                        data-idx={bi}
                        onClick={() => setCursor(bi)}
                        className={`rounded px-1 text-left transition-colors ${
                          cursor === bi
                            ? "bg-chrono-accent/40 text-white font-semibold"
                            : "text-gray-400 hover:bg-white/10"
                        }`}
                      >
                        {black.move_san}
                      </button>
                    ) : (
                      <span />
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}

          {!loading && game.result && (
            <div className="mt-3 pt-3 border-t border-chrono-border text-center text-xs text-gray-500 shrink-0">
              {resultLabel}
              {game.winner_id && (
                <span className="ml-1 text-gray-400">
                  · {game.winner_id === game.white_player_id ? game.white_username : game.black_username} wins
                </span>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
