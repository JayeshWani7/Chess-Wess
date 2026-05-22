import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { api, type GameHistoryEntry } from "../utils/api";
import { useAuthStore } from "../store/authStore";

const FILTERS = ["all", "win", "loss", "draw"] as const;

export default function GameHistoryPage() {
  const { token, userId } = useAuthStore();
  const [history, setHistory] = useState<GameHistoryEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [filter, setFilter] = useState<(typeof FILTERS)[number]>("all");

  useEffect(() => {
    if (!token) return;
    setLoading(true);
    api.listMyGames(token)
      .then((list) => setHistory(list ?? []))
      .catch(() => setHistory([]))
      .finally(() => setLoading(false));
  }, [token]);

  const filtered = useMemo(() => {
    if (filter === "all") return history;
    return history.filter((game) => {
      const isDraw = game.result === "stalemate" || game.result === "draw";
      const didWin = game.winner_id === userId;
      if (filter === "draw") return isDraw;
      if (filter === "win") return !isDraw && didWin;
      return !isDraw && !didWin;
    });
  }, [filter, history, userId]);

  return (
    <div className="space-y-6">
      <header className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
        <div>
          <p className="text-xs uppercase tracking-[0.3em] text-moss">Archive</p>
          <h1 className="text-3xl font-display text-ink">Game History</h1>
        </div>
        <div className="flex flex-wrap gap-2">
          {FILTERS.map((tab) => (
            <button
              key={tab}
              onClick={() => setFilter(tab)}
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
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-display text-ink">Recent games</h2>
          <span className="text-xs text-moss">{filtered.length} total</span>
        </div>

        {loading ? (
          <p className="text-sm text-moss">Loading games…</p>
        ) : filtered.length === 0 ? (
          <p className="text-sm text-moss">No completed games yet.</p>
        ) : (
          <div className="space-y-3">
            {filtered.map((game) => {
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
              const date = dateSource ? new Date(dateSource).toLocaleDateString() : "Unknown";

              return (
                <div
                  key={game.id}
                  className="flex flex-col gap-3 rounded-xl border border-line bg-paper px-4 py-3 md:flex-row md:items-center md:justify-between"
                >
                  <div className="space-y-1">
                    <div className="flex items-center gap-3">
                      <span className={`text-sm font-semibold ${outcomeTone}`}>
                        {outcomeLabel}
                      </span>
                      <p className="text-sm text-ink">
                        {isWhite ? "White" : "Black"} · {myName} vs {oppName}
                      </p>
                    </div>
                    <p className="text-xs text-moss">
                      {game.result ?? "Completed"} · {game.time_control === 0 ? "Unlimited" : `${Math.round(game.time_control / 60)} min`} · {date}
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    <Link to={`/review/${game.id}`} className="btn-outline text-xs">
                      Review
                    </Link>
                    <button className="btn-ghost text-xs">Share</button>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
