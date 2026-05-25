import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api, type GameInfo } from "../utils/api";
import { useAuthStore } from "../store/authStore";

export default function HomePage() {
  const { token, username } = useAuthStore();
  const [openGames, setOpenGames] = useState<GameInfo[]>([]);

  useEffect(() => {
    if (!token) return;
    api.listGames(token)
      .then((games) => setOpenGames(games ?? []))
      .catch(() => setOpenGames([]));
  }, [token]);

  return (
    <div className="space-y-10">
      <section className="grid gap-8 lg:grid-cols-[1.2fr_0.8fr] items-center">
        <div className="space-y-5">
          <p className="text-xs uppercase tracking-[0.4em] text-moss">
            Welcome, {username}
          </p>
          <h1 className="text-4xl font-display text-pine leading-tight">
            Branch your fate across timelines.
          </h1>
          <p className="text-lg text-ink/70">
            ChessWess turns every move into a new reality. Rewind, fork, and win
            in worlds that only you can see.
          </p>
          <div className="flex flex-wrap gap-3">
            <Link to="/lobby" className="btn-primary">
              Start a Game
            </Link>
            <Link to="/history" className="btn-outline">
              Review Past Battles
            </Link>
          </div>
        </div>

        <div className="card p-6">
          <div className="space-y-4">
            <div>
              <p className="text-xs uppercase tracking-[0.3em] text-moss">
                Quick Actions
              </p>
              <h2 className="text-2xl font-display text-ink">
                Jump into the multiverse
              </h2>
            </div>
            <div className="grid gap-3">
              <div className="rounded-xl border border-line bg-paper p-4">
                <p className="text-sm font-semibold text-ink">Quick Match</p>
                <p className="text-xs text-moss">
                  Find a human opponent in seconds.
                </p>
              </div>
              <div className="rounded-xl border border-line bg-paper p-4">
                <p className="text-sm font-semibold text-ink">Play vs Bot</p>
                <p className="text-xs text-moss">
                  Practice with adaptive timeline AI.
                </p>
              </div>
              <div className="rounded-xl border border-line bg-paper p-4">
                <p className="text-sm font-semibold text-ink">Create Room</p>
                <p className="text-xs text-moss">
                  Invite friends to shape alternate realities.
                </p>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section className="grid gap-6 lg:grid-cols-[1.2fr_0.8fr]">
        <div className="card">
          <div className="flex items-center justify-between mb-4">
            <div>
              <p className="text-xs uppercase tracking-[0.3em] text-moss">Live Lobby</p>
              <h3 className="text-xl font-display text-ink">Open games</h3>
            </div>
            <Link to="/lobby" className="text-sm text-pine hover:text-pine/80">
              View lobby
            </Link>
          </div>
          {openGames.length === 0 ? (
            <p className="text-sm text-moss">No open games right now.</p>
          ) : (
            <div className="space-y-2">
              {openGames.slice(0, 4).map((game) => (
                <div
                  key={game.id}
                  className="flex items-center justify-between rounded-xl border border-line bg-paper px-4 py-3"
                >
                  <div>
                    <p className="text-sm font-semibold text-ink">
                      {game.white_player_id ? "Waiting for Black" : "Waiting for White"}
                    </p>
                    <p className="text-xs text-moss">
                      {game.time_control === 0 ? "Unlimited" : `${Math.round(game.time_control / 60)} min`} · {game.id.slice(0, 8)}
                    </p>
                  </div>
                  <Link to="/lobby" className="btn-outline text-xs">
                    Join in lobby
                  </Link>
                </div>
              ))}
            </div>
          )}
        </div>

        <div className="card p-6">
          <div className="flex flex-col gap-4">
            <div className="space-y-2">
              <p className="text-xs uppercase tracking-[0.3em] text-moss">Featured Replay</p>
              <h3 className="text-xl font-display text-ink">The Sacrificial Queen</h3>
            </div>
            <p className="text-sm text-ink/70 leading-relaxed">
              A bold rewind that flips the entire board. Explore decisive
              branches and see how the multiverse collapses.
            </p>
            <Link to="/history" className="btn-primary inline-flex w-fit text-sm">
              Watch replay
            </Link>
          </div>
        </div>
      </section>
    </div>
  );
}
