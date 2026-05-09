import { useAuthStore } from "./store/authStore";
import AuthPage from "./pages/AuthPage";
import LobbyPage from "./pages/LobbyPage";
import GamePage from "./pages/GamePage";
import { useGameStore } from "./store/gameStore";

export default function App() {
  const token = useAuthStore((s) => s.token);
  const activeGameId = useGameStore((s) => s.activeGameId);

  if (!token) return <AuthPage />;
  if (activeGameId) return <GamePage />;
  return <LobbyPage />;
}
