import { useEffect, useRef } from "react";
import { useGameStore } from "../../store/gameStore";

export default function MoveHistory() {
  const moves = useGameStore((s) => s.moves);
  const bottomRef = useRef<HTMLDivElement>(null);

  // Auto-scroll to the latest move
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth", block: "nearest" });
  }, [moves.length]);

  // Group flat move list into pairs: [[white, black?], ...]
  // move_number is 1-based and alternates w/b, so pair index = Math.ceil(n/2) - 1
  const pairs: Array<{ moveNum: number; white: string; black: string | null }> = [];
  for (let i = 0; i < moves.length; i += 2) {
    pairs.push({
      moveNum: Math.ceil(moves[i].moveNumber / 2),
      white: moves[i].moveSan,
      black: moves[i + 1]?.moveSan ?? null,
    });
  }

  const lastMoveIndex = moves.length - 1; // 0-based index of the last move

  return (
    <div className="card flex flex-col min-h-0" style={{ maxHeight: "320px" }}>
      <h3 className="text-sm font-semibold text-gray-400 mb-2 shrink-0">Move History</h3>

      {pairs.length === 0 ? (
        <p className="text-gray-600 text-xs">No moves yet</p>
      ) : (
        <div className="overflow-y-auto flex-1 pr-1">
          {/* Header row */}
          <div className="grid grid-cols-[2rem_1fr_1fr] gap-x-2 px-1 mb-1 text-xs text-gray-500 font-semibold uppercase tracking-wide">
            <span>#</span>
            <span>White</span>
            <span>Black</span>
          </div>

          <div className="space-y-0.5 font-mono text-sm">
            {pairs.map(({ moveNum, white, black }, pairIdx) => {
              // Flat indices: white = pairIdx*2, black = pairIdx*2+1
              const whiteIdx = pairIdx * 2;
              const blackIdx = pairIdx * 2 + 1;
              const whiteIsLast = whiteIdx === lastMoveIndex;
              const blackIsLast = blackIdx === lastMoveIndex;

              return (
                <div
                  key={pairIdx}
                  className="grid grid-cols-[2rem_1fr_1fr] gap-x-2 rounded px-1 py-0.5 hover:bg-white/5"
                >
                  <span className="text-gray-500 text-right">{moveNum}.</span>

                  <span
                    className={`rounded px-1 transition-colors ${
                      whiteIsLast
                        ? "bg-chrono-accent/30 text-white font-semibold"
                        : "text-gray-200"
                    }`}
                  >
                    {white}
                  </span>

                  <span
                    className={`rounded px-1 transition-colors ${
                      blackIsLast && black
                        ? "bg-chrono-accent/30 text-white font-semibold"
                        : "text-gray-400"
                    }`}
                  >
                    {black ?? ""}
                  </span>
                </div>
              );
            })}
          </div>

          {/* Scroll anchor */}
          <div ref={bottomRef} />
        </div>
      )}
    </div>
  );
}
