import { useEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { Chess } from "chess.js";
import { api, type GameHistoryEntry, type GameTimelineResponse } from "../utils/api";
import { useAuthStore } from "../store/authStore";

const FILES = ["a", "b", "c", "d", "e", "f", "g", "h"];
const RANKS = ["8", "7", "6", "5", "4", "3", "2", "1"];

const PIECES: Record<string, string> = {
  wk: "♔", wq: "♕", wr: "♖", wb: "♗", wn: "♘", wp: "♙",
  bk: "♚", bq: "♛", br: "♜", bb: "♝", bn: "♞", bp: "♟",
};

interface Props {
  game?: GameHistoryEntry;
  onBack?: () => void;
}

interface ReviewMove {
  id: string;
  move_number: number;
  move_san: string;
  move_uci: string;
  fen_after: string;
}

export default function GameReviewPage({ game, onBack }: Props) {
  const { token, userId } = useAuthStore();
  const { gameId } = useParams();
  const navigate = useNavigate();
  const [resolvedGame, setResolvedGame] = useState<GameHistoryEntry | null>(game ?? null);
  const [moves, setMoves] = useState<ReviewMove[]>([]);
  const [cursor, setCursor] = useState(-1);
  const [loading, setLoading] = useState(true);
  const moveListRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!token) return;
    if (resolvedGame) return;
    if (!gameId) return;
    api.getGame(token, gameId)
      .then(async (gameInfo) => {
        const [whiteUser, blackUser] = await Promise.all([
          gameInfo.white_player_id
            ? api.getUser(token, gameInfo.white_player_id).catch(() => null)
            : Promise.resolve(null),
          gameInfo.black_player_id
            ? api.getUser(token, gameInfo.black_player_id).catch(() => null)
            : Promise.resolve(null),
        ]);
        setResolvedGame({
          ...gameInfo,
          white_username: whiteUser?.username ?? "White",
          black_username: blackUser?.username ?? "Black",
        } as GameHistoryEntry);
      })
      .catch(() => setResolvedGame(null));
  }, [token, resolvedGame, gameId]);

  useEffect(() => {
    if (!token || !resolvedGame) return;
    api.getGameTimeline(token, resolvedGame.id).then((data: GameTimelineResponse) => {
      const timelineId = data.active_timeline_id ?? data.timelines[0]?.timeline_id ?? null;
      const timeline = timelineId
        ? data.timelines.find((t) => t.timeline_id === timelineId) ?? data.timelines[0]
        : data.timelines[0];

      const nodes = [...(timeline?.nodes ?? [])].sort((a, b) => a.turn_number - b.turn_number);
      const reviewMoves = nodes
        .filter((n) => n.move?.uci && n.move?.san)
        .map((n) => ({
          id: n.id,
          move_number: n.turn_number,
          move_san: n.move?.san ?? "",
          move_uci: n.move?.uci ?? "",
          fen_after: n.board_state,
        }));

      if (reviewMoves.length > 0) {
        setMoves(reviewMoves);
        setCursor(reviewMoves.length - 1);
        setLoading(false);
      } else {
        // Fallback: game was played in standard mode without timeline nodes
        return api.getGameMoves(token, resolvedGame.id).then((rawMoves) => {
          const fallback = rawMoves.map((m) => ({
            id: m.id,
            move_number: m.move_number,
            move_san: m.move_san,
            move_uci: m.move_uci,
            fen_after: m.fen_after,
          }));
          setMoves(fallback);
          setCursor(fallback.length - 1);
          setLoading(false);
        });
      }
    }).catch(() => {
      // Timeline endpoint failed — fall back to game_moves table
      api.getGameMoves(token, resolvedGame.id).then((rawMoves) => {
        const fallback = rawMoves.map((m) => ({
          id: m.id,
          move_number: m.move_number,
          move_san: m.move_san,
          move_uci: m.move_uci,
          fen_after: m.fen_after,
        }));
        setMoves(fallback);
        setCursor(fallback.length - 1);
      }).catch(() => {
        setMoves([]);
        setCursor(-1);
      }).finally(() => setLoading(false));
    });
  }, [token, resolvedGame]);

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
    cursor >= 0 && moves[cursor].move_uci.length >= 4
      ? [moves[cursor].move_uci.slice(0, 2), moves[cursor].move_uci.slice(2, 4)]
      : [];

  const flipped =
    resolvedGame?.black_player_id === userId &&
    resolvedGame?.white_player_id !== userId;
  const ranks = flipped ? [...RANKS].reverse() : RANKS;
  const fileOrder = flipped ? [...FILES].reverse() : FILES;

  const resultLabel = (() => {
    const r = resolvedGame?.result;
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

  const didWin = resolvedGame?.winner_id === userId;
  const isDraw = resolvedGame?.result === "stalemate" || resolvedGame?.result === "draw";
  const outcomeLabel = isDraw ? "Draw" : didWin ? "Victory" : "Defeat";
  const outcomeColor = isDraw
    ? "text-moss"
    : didWin
    ? "text-leaf"
    : "text-rust";

  const pairs: Array<{ num: number; white: ReviewMove | null; black: ReviewMove | null; wi: number; bi: number }> = [];
  for (let i = 0; i < moves.length; i += 2) {
    pairs.push({ num: i / 2 + 1, white: moves[i] ?? null, black: moves[i + 1] ?? null, wi: i, bi: i + 1 });
  }

  if (!resolvedGame) {
    return (
      <div className="card text-sm text-moss">
        Game not found. <Link to="/history" className="text-pine">Back to history</Link>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <button
          onClick={onBack ?? (() => navigate("/history"))}
          className="btn-ghost text-sm"
        >
          ← Back
        </button>
        <h1 className="text-xl font-display text-ink">Game Review</h1>
        <div className="text-right">
          <span className={`font-bold text-sm ${outcomeColor}`}>{outcomeLabel}</span>
          {resultLabel && <span className="text-moss text-xs ml-2">· {resultLabel}</span>}
        </div>
      </div>

      <div className="flex items-center justify-between mb-4 px-1">
        <div className="flex items-center gap-2">
          <span className="text-lg">⬜</span>
          <span className="font-semibold text-sm">{resolvedGame.white_username}</span>
        </div>
        <span className="text-moss text-xs">vs</span>
        <div className="flex items-center gap-2">
          <span className="font-semibold text-sm">{resolvedGame.black_username}</span>
          <span className="text-lg">⬛</span>
        </div>
      </div>

      <div className="flex flex-col lg:flex-row gap-6 items-start justify-center">
        <div className="flex flex-col items-center gap-2">
          <div className="relative select-none">
            <div
              className="grid border-2 border-line rounded-sm overflow-hidden shadow-2xl"
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
                <span key={f} className="flex-1 text-center text-xs text-moss">{f}</span>
              ))}
            </div>

            <div
              className="absolute top-0 left-0 flex flex-col"
              style={{ height: "min(80vw, 480px)", transform: "translateX(-16px)" }}
            >
              {ranks.map((r) => (
                <span key={r} className="flex-1 flex items-center text-xs text-moss">{r}</span>
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
            <span className="text-xs text-moss w-20 text-center tabular-nums">
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
          <p className="text-xs text-moss">Use ← → arrow keys to navigate</p>
        </div>

        <div className="card flex flex-col w-full lg:w-64" style={{ maxHeight: "520px" }}>
          <h3 className="text-sm font-semibold text-moss mb-2 shrink-0">
            Moves · {moves.length} total
          </h3>

          {loading ? (
            <p className="text-moss text-xs">Loading…</p>
          ) : (
            <div className="overflow-y-auto flex-1 pr-1" ref={moveListRef}>
              <div className="grid grid-cols-[2rem_1fr_1fr] gap-x-2 px-1 mb-1 text-xs text-moss font-semibold uppercase tracking-wide shrink-0">
                <span>#</span><span>White</span><span>Black</span>
              </div>

              <div className="space-y-0.5 font-mono text-sm">
                {pairs.map(({ num, white, black, wi, bi }) => (
                  <div
                    key={num}
                    className="grid grid-cols-[2rem_1fr_1fr] gap-x-2 rounded px-1 py-0.5"
                  >
                    <span className="text-moss text-right">{num}.</span>

                    {white && (
                      <button
                        data-idx={wi}
                        onClick={() => setCursor(wi)}
                        className={`rounded px-1 text-left transition-colors ${
                          cursor === wi
                            ? "bg-mist text-pine font-semibold"
                            : "text-ink hover:bg-mist"
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
                            ? "bg-mist text-pine font-semibold"
                            : "text-moss hover:bg-mist"
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

          {!loading && resolvedGame.result && (
            <div className="mt-3 pt-3 border-t border-line text-center text-xs text-moss shrink-0">
              {resultLabel}
              {resolvedGame.winner_id && (
                <span className="ml-1 text-moss">
                  · {resolvedGame.winner_id === resolvedGame.white_player_id ? resolvedGame.white_username : resolvedGame.black_username} wins
                </span>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
