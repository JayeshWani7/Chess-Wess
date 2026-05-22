import { Navigate, Route, Routes } from "react-router-dom";
import { useAuthStore } from "./store/authStore";
import { useGameStore } from "./store/gameStore";
import AppShell from "./components/Layout/AppShell";
import AuthPage from "./pages/AuthPage";
import HomePage from "./pages/HomePage";
import LobbyPage from "./pages/LobbyPage";
import GamePage from "./pages/GamePage";
import GameHistoryPage from "./pages/GameHistoryPage";
import GameReviewPage from "./pages/GameReviewPage";

export default function App() {
  const token = useAuthStore((s) => s.token);
  const activeGameId = useGameStore((s) => s.activeGameId);

  return (
    <Routes>
      <Route
        path="/auth"
        element={token ? <Navigate to="/" replace /> : <AuthPage />}
      />
      <Route
        element={token ? <AppShell /> : <Navigate to="/auth" replace />}
      >
        <Route index element={<HomePage />} />
        <Route path="/lobby" element={<LobbyPage />} />
        <Route
          path="/game"
          element={activeGameId ? <GamePage /> : <Navigate to="/lobby" replace />}
        />
        <Route path="/history" element={<GameHistoryPage />} />
        <Route path="/review/:gameId" element={<GameReviewPage />} />
      </Route>
      <Route
        path="*"
        element={<Navigate to={token ? "/" : "/auth"} replace />}
      />
    </Routes>
  );
}
