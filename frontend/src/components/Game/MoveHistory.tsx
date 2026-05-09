import { useGameStore } from "../../store/gameStore";

export default function MoveHistory() {
  const moves = useGameStore((s) => s.moves);

  // Group into pairs: [white, black]
  const pairs: Array<[string, string | null]> = [];
  for (let i = 0; i < moves.length; i += 2) {
    pairs.push([moves[i].moveSan, moves[i + 1]?.moveSan ?? null]);
  }

  return (
    <div className="card flex-1 overflow-y-auto min-h-0">
      <h3 className="text-sm font-semibold text-gray-400 mb-3">Move History</h3>
      {pairs.length === 0 ? (
        <p className="text-gray-600 text-xs">No moves yet</p>
      ) : (
        <div className="space-y-0.5 font-mono text-sm">
          {pairs.map(([white, black], i) => (
            <div key={i} className="flex gap-2 hover:bg-chrono-border/30 rounded px-1">
              <span className="text-gray-500 w-6 text-right shrink-0">{i + 1}.</span>
              <span className="text-white w-16">{white}</span>
              <span className="text-gray-300">{black ?? ""}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
