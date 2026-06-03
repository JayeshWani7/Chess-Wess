import { useEffect, useState, useCallback } from "react";
import { Link } from "react-router-dom";
import { api, type GameHistoryEntry } from "../utils/api";
import { useAuthStore } from "../store/authStore";

const FILTERS = ["all", "win", "loss", "draw"] as const;
type Filter = (typeof FILTERS)[number];
const PAGE_SIZE = 10;

export default function GameHistoryPage() {
  const { token, userId } = useAuthStore();

  const [games, setGames] = useState<GameHistoryEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [filter, setFilter] = useState<Filter>("all");
  const [loading, setLoading] = useState(false);
  const [copiedId, setCopiedId] = useState<string | null>(null);

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  // Fetch whenever page or filter changes
  useEffect(() => {
    if (!token) return;
    setLoading(true);
    api
      .listMyGames(token, page, PAGE_SIZE, filter)
      .then((res) => {
        setGames(res.games ?? []);
        setTotal(res.total ?? 0);
      })
      .catch(() => {
        setGames([]);
        setTotal(0);
      })
      .finally(() => setLoading(false));
  }, [token, page, filter]);

  // Reset to page 1 when filter changes
  function handleFilterChange(f: Filter) {
    setFilter(f);
    setPage(1);
  }

  const handleShare = useCallback(async (gameId: string) => {
    const url = `${window.location.origin}/review/${gameId}`;
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(url);
      } else {
        const ta = document.createElement("textarea");
        ta.value = url;
        ta.style.cssText = "position:fixed;opacity:0";
        document.body.appendChild(ta);
        ta.focus();
        ta.select();
        document.execCommand("copy");
        document.body.removeChild(ta);
      }
      setCopiedId(gameId);
      setTimeout(() => setCopiedId((prev) => (prev === gameId ? null : prev)), 2000);
    } catch {
      window.open(url, "_blank", "noopener");
    }
  }, []);

  return (
    <div className="space-y-6">
      {/* Header */}
      <header className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
        <div>
          <p className="text-xs uppercase tracking-[0.3em] text-moss">Archive</p>
          <h1 className="text-3xl font-display text-ink">Game History</h1>
        </div>
        <div className="flex flex-wrap gap-2">
          {FILTERS.map((tab) => (
            <button
              key={tab}
              onClick={() => handleFilterChange(tab)}
              className={`rounded-full px-4 py-2 text-xs font-semibold transition-colors border ${
                filter === tab
                  ? "border-gold bg-mist text-pine"
                  : "border-line text-ink/70 hover:text-ink hover:bg-mist"
              }`}
            >
              {tab.toUpperCase()}
            </button>
          ))}
        </div>
      </header>

      <div className="card">
        {/* Table header */}
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-display text-ink">Recent games</h2>
          <span className="text-xs text-moss">
            {total > 0
              ? `${(page - 1) * PAGE_SIZE + 1}–${Math.min(page * PAGE_SIZE, total)} of ${total}`
              : "0 games"}
          </span>
        </div>

        {/* Game rows */}
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <span className="text-sm text-moss animate-pulse">Loading…</span>
          </div>
        ) : games.length === 0 ? (
          <p className="text-sm text-moss py-8 text-center">No games found.</p>
        ) : (
          <div className="space-y-3">
            {games.map((game) => {
              const isWhite = game.white_player_id === userId;
              const myName = isWhite ? game.white_username : game.black_username;
              const oppName = isWhite ? game.black_username : game.white_username;
              const isDraw = game.result === "stalemate" || game.result === "draw";
              const didWin = game.winner_id === userId;
              const outcomeLabel = isDraw ? "Draw" : didWin ? "Win" : "Loss";
              const outcomeTone = isDraw
                ? "text-moss"
                : didWin
                ? "text-leaf"
                : "text-rust";
              const dateSource = game.updated_at ?? game.created_at;
              const date = dateSource
                ? new Date(dateSource).toLocaleDateString()
                : "Unknown";
              const timeLabel =
                game.time_control === 0
                  ? "Unlimited"
                  : `${Math.round(game.time_control / 60)} min`;

              return (
                <div
                  key={game.id}
                  className="flex flex-col gap-3 rounded-xl border border-line bg-paper px-4 py-3 md:flex-row md:items-center md:justify-between"
                >
                  <div className="space-y-1">
                    <div className="flex items-center gap-3">
                      <span className={`text-sm font-semibold w-8 ${outcomeTone}`}>
                        {outcomeLabel}
                      </span>
                      <p className="text-sm text-ink">
                        {isWhite ? "⬜" : "⬛"} {myName}{" "}
                        <span className="text-moss">vs</span>{" "}
                        {isWhite ? "⬛" : "⬜"} {oppName}
                      </p>
                    </div>
                    <p className="text-xs text-moss">
                      {game.result ?? "Completed"} · {timeLabel} · {date}
                    </p>
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    <Link to={`/review/${game.id}`} className="btn-outline text-xs">
                      Review
                    </Link>
                    <button
                      onClick={() => handleShare(game.id)}
                      className={`btn-ghost text-xs transition-colors ${
                        copiedId === game.id ? "text-leaf border-leaf/40" : ""
                      }`}
                      title="Copy review link"
                    >
                      {copiedId === game.id ? "✓ Copied!" : "Share"}
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        )}

        {/* Pagination controls */}
        {totalPages > 1 && (
          <div className="flex items-center justify-center gap-1 mt-6 flex-wrap">
            {/* Previous */}
            <button
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page === 1 || loading}
              className="px-3 py-1.5 rounded-lg text-sm border border-line text-ink/70 hover:text-ink hover:bg-mist disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
            >
              ←
            </button>

            {/* Page numbers */}
            {buildPageRange(page, totalPages).map((item, idx) =>
              item === "…" ? (
                <span key={`ellipsis-${idx}`} className="px-2 text-moss text-sm select-none">
                  …
                </span>
              ) : (
                <button
                  key={item}
                  onClick={() => setPage(item as number)}
                  disabled={loading}
                  className={`min-w-[2rem] px-3 py-1.5 rounded-lg text-sm border transition-colors disabled:cursor-not-allowed ${
                    item === page
                      ? "border-gold bg-mist text-pine font-semibold"
                      : "border-line text-ink/70 hover:text-ink hover:bg-mist"
                  }`}
                >
                  {item}
                </button>
              )
            )}

            {/* Next */}
            <button
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              disabled={page === totalPages || loading}
              className="px-3 py-1.5 rounded-lg text-sm border border-line text-ink/70 hover:text-ink hover:bg-mist disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
            >
              →
            </button>
          </div>
        )}

        {/* Per-page info */}
        {total > 0 && (
          <p className="text-center text-xs text-moss mt-3">
            Page {page} of {totalPages}
          </p>
        )}
      </div>
    </div>
  );
}

/**
 * Builds a compact page-number range with ellipsis.
 * Always shows first, last, and up to 2 pages around the current page.
 * e.g. page=6, total=12 → [1, "…", 5, 6, 7, "…", 12]
 */
function buildPageRange(current: number, total: number): (number | "…")[] {
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1);

  const pages = new Set<number>();
  pages.add(1);
  pages.add(total);
  for (let d = -2; d <= 2; d++) {
    const p = current + d;
    if (p >= 1 && p <= total) pages.add(p);
  }

  const sorted = Array.from(pages).sort((a, b) => a - b);
  const result: (number | "…")[] = [];
  let prev = 0;
  for (const p of sorted) {
    if (p - prev > 1) result.push("…");
    result.push(p);
    prev = p;
  }
  return result;
}
