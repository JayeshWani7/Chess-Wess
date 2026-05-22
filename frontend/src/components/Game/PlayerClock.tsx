import { useEffect, useRef } from "react";
import { useGameStore } from "../../store/gameStore";

interface PlayerClockProps {
  color: "w" | "b";
  username: string;
}

export default function PlayerClock({ color, username }: PlayerClockProps) {
  const { chess, whiteTime, blackTime, status, setTimers, setGameOver } =
    useGameStore();
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const time = color === "w" ? whiteTime : blackTime;
  const isActive =
    status === "active" && chess.turn() === color;
  const isUnlimited = time === 0 && whiteTime === 0 && blackTime === 0;

  useEffect(() => {
    if (!isActive || isUnlimited) {
      if (intervalRef.current) clearInterval(intervalRef.current);
      return;
    }

    intervalRef.current = setInterval(() => {
      const { whiteTime, blackTime } = useGameStore.getState();
      const newWhite = color === "w" ? Math.max(0, whiteTime - 1) : whiteTime;
      const newBlack = color === "b" ? Math.max(0, blackTime - 1) : blackTime;
      setTimers(newWhite, newBlack);

      if ((color === "w" && newWhite === 0) || (color === "b" && newBlack === 0)) {
        setGameOver("timeout", null);
      }
    }, 1000);

    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [isActive, isUnlimited, color, setTimers, setGameOver]);

  function formatTime(secs: number) {
    if (isUnlimited) return "∞";
    const m = Math.floor(secs / 60);
    const s = secs % 60;
    return `${m}:${String(s).padStart(2, "0")}`;
  }

  const urgent = !isUnlimited && time <= 30;

  return (
    <div
      className={`flex items-center justify-between px-4 py-3 rounded-lg border transition-colors ${
        isActive
          ? "border-leaf bg-mist"
          : "border-line bg-panel"
      }`}
    >
      <div className="flex items-center gap-2">
        <span className="text-lg">{color === "w" ? "⬜" : "⬛"}</span>
        <span className="font-semibold text-sm text-ink">{username}</span>
      </div>
      <span
        className={`font-mono text-xl font-bold tabular-nums ${
          urgent ? "text-rust animate-pulse" : "text-ink"
        }`}
      >
        {formatTime(time)}
      </span>
    </div>
  );
}
