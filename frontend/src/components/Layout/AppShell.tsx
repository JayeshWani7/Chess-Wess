import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { useAuthStore } from "../../store/authStore";
import { useGameStore } from "../../store/gameStore";

const navItem = ({ isActive }: { isActive: boolean }) =>
  `px-3 py-2 rounded-lg text-sm font-semibold transition-colors border ${
    isActive
      ? "border-gold bg-mist text-pine"
      : "border-transparent text-ink/70 hover:text-ink hover:bg-mist"
  }`;

export default function AppShell() {
  const { username, logout } = useAuthStore();
  const activeGameId = useGameStore((s) => s.activeGameId);
  const navigate = useNavigate();

  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-20 border-b border-line/70 bg-paper/80 backdrop-blur">
        <div className="mx-auto flex w-full max-w-6xl items-center justify-between px-4 py-3">
          <div className="flex items-center gap-4">
            <button
              className="text-left"
              onClick={() => navigate("/")}
              aria-label="ChessWess home"
            >
              <p className="text-xs uppercase tracking-[0.3em] text-moss">ChessWess</p>
              <p className="text-lg font-display text-pine">Multiverse Chess</p>
            </button>
            <nav className="hidden items-center gap-1 md:flex">
              <NavLink to="/" className={navItem}>
                Home
              </NavLink>
              <NavLink to="/lobby" className={navItem}>
                Play
              </NavLink>
              <NavLink to="/history" className={navItem}>
                History
              </NavLink>
              {activeGameId && (
                <NavLink to="/game" className={navItem}>
                  Game
                </NavLink>
              )}
            </nav>
          </div>

          <div className="flex items-center gap-3">
            {activeGameId && (
              <button
                onClick={() => navigate("/game")}
                className="hidden rounded-full border border-leaf bg-leaf/10 px-3 py-1 text-xs font-semibold text-pine md:inline-flex"
              >
                Active game
              </button>
            )}
            <div className="text-right">
              <p className="text-xs text-moss">Signed in as</p>
              <p className="text-sm font-semibold text-ink">@{username}</p>
            </div>
            <button onClick={logout} className="btn-outline text-sm">
              Sign out
            </button>
          </div>
        </div>
        <div className="md:hidden border-t border-line/60 bg-paper/90">
          <nav className="mx-auto flex w-full max-w-6xl items-center justify-around px-4 py-2">
            <NavLink to="/" className={navItem}>
              Home
            </NavLink>
            <NavLink to="/lobby" className={navItem}>
              Play
            </NavLink>
            <NavLink to="/history" className={navItem}>
              History
            </NavLink>
            {activeGameId && (
              <NavLink to="/game" className={navItem}>
                Game
              </NavLink>
            )}
          </nav>
        </div>
      </header>

      <main className="mx-auto w-full max-w-6xl px-4 py-8">
        <Outlet />
      </main>
    </div>
  );
}
