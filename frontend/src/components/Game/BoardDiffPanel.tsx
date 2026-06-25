import { useMemo } from "react";

interface BoardDiffPanelProps {
  fenA: string;
  fenB: string;
  labelA?: string;
  labelB?: string;
  onClose: () => void;
}

const FILES = ["a", "b", "c", "d", "e", "f", "g", "h"];
const RANKS = ["8", "7", "6", "5", "4", "3", "2", "1"];

const PIECES: Record<string, string> = {
  wk: "♔", wq: "♕", wr: "♖", wb: "♗", wn: "♘", wp: "♙",
  bk: "♚", bq: "♛", br: "♜", bb: "♝", bn: "♞", bp: "♟",
};

interface DiffSquare {
  square: string;
  pieceA: { type: string; color: "w" | "b" } | null;
  pieceB: { type: string; color: "w" | "b" } | null;
  status: "same" | "moved" | "captured" | "added";
  isLight: boolean;
}

function parseFenPart(boardPart: string): Record<string, { type: string; color: "w" | "b" }> {
  const rows = boardPart.split("/");
  const map: Record<string, { type: string; color: "w" | "b" }> = {};

  for (let r = 0; r < 8; r++) {
    const rowStr = rows[r] ?? "";
    const rank = RANKS[r];
    let fileIdx = 0;
    for (let i = 0; i < rowStr.length; i++) {
      const char = rowStr[i];
      if (char >= "1" && char <= "8") {
        fileIdx += parseInt(char, 10);
      } else {
        const file = FILES[fileIdx];
        const isWhite = char === char.toUpperCase();
        map[`${file}${rank}`] = {
          type: char.toLowerCase(),
          color: isWhite ? "w" : "b",
        };
        fileIdx++;
      }
    }
  }
  return map;
}

export default function BoardDiffPanel({ fenA, fenB, labelA = "Position A", labelB = "Position B", onClose }: BoardDiffPanelProps) {
  const diffSquares = useMemo(() => {
    const partA = fenA.split(" ")[0] ?? "";
    const partB = fenB.split(" ")[0] ?? "";

    const mapA = parseFenPart(partA);
    const mapB = parseFenPart(partB);

    const squares: DiffSquare[] = [];

    for (const rank of RANKS) {
      for (const file of FILES) {
        const square = `${file}${rank}`;
        const pA = mapA[square] ?? null;
        const pB = mapB[square] ?? null;

        let status: DiffSquare["status"] = "same";
        if (pA && !pB) {
          status = "captured";
        } else if (!pA && pB) {
          status = "added";
        } else if (pA && pB && (pA.type !== pB.type || pA.color !== pB.color)) {
          status = "moved";
        }

        const isLight = (FILES.indexOf(file) + RANKS.indexOf(rank)) % 2 === 0;

        squares.push({
          square,
          pieceA: pA,
          pieceB: pB,
          status,
          isLight,
        });
      }
    }
    return squares;
  }, [fenA, fenB]);

  function renderMiniBoard(which: "A" | "B") {
    return (
      <div
        className="grid border border-line rounded-lg overflow-hidden shadow-md"
        style={{
          gridTemplateColumns: "repeat(8, 1fr)",
          gridTemplateRows: "repeat(8, 1fr)",
          width: "240px",
          height: "240px",
        }}
      >
        {diffSquares.map((sq) => {
          const piece = which === "A" ? sq.pieceA : sq.pieceB;
          const key = piece ? `${piece.color}${piece.type}` : "";
          const symbol = PIECES[key] ?? "";
          
          let highlightClass = "";
          if (sq.status !== "same") {
            if (sq.status === "captured") {
              highlightClass = which === "A" ? "bg-red-200/60" : "";
            } else if (sq.status === "added") {
              highlightClass = which === "B" ? "bg-emerald-200/60" : "";
            } else if (sq.status === "moved") {
              highlightClass = "bg-amber-100/60";
            }
          }

          const baseBg = sq.isLight ? "bg-[#f2e8d5]" : "bg-[#b2a991]";

          return (
            <div
              key={`${which}-${sq.square}`}
              className={`relative flex items-center justify-center ${highlightClass || baseBg}`}
              style={{ width: "30px", height: "30px" }}
              title={`${sq.square}: ${sq.status !== "same" ? `Diff (${sq.status})` : "Identical"}`}
            >
              {symbol && (
                <span
                  className="text-lg leading-none select-none"
                  style={{
                    color: piece?.color === "w" ? "#fff" : "#1b1e1a",
                    WebkitTextStroke: piece?.color === "w" ? "0.5px #333" : "1px #000",
                    textShadow: piece?.color === "w" ? "0 0 1px #000, 0 0 1px #000" : "none",
                  }}
                >
                  {symbol}
                </span>
              )}
            </div>
          );
        })}
      </div>
    );
  }

  return (
    <div className="card space-y-4 border border-purple-300 bg-purple-50/10 p-4">
      <div className="flex items-center justify-between">
        <div>
          <h4 className="text-sm font-semibold text-ink flex items-center gap-1.5">
            <span className="flex h-5 w-5 items-center justify-center rounded-full bg-purple-100 text-purple-600 text-xs">
              ⌥
            </span>
            Timeline Position Diffing
          </h4>
          <p className="text-xs text-moss">Comparing difference in pieces and square layouts side-by-side.</p>
        </div>
        <button onClick={onClose} className="btn-ghost text-xs py-1 px-2.5 rounded-lg border border-line">
          Close Diff
        </button>
      </div>

      <div className="flex flex-wrap gap-6 justify-center items-center py-2">
        <div className="flex flex-col items-center gap-1.5">
          <span className="text-xs font-semibold text-moss uppercase tracking-wider">{labelA}</span>
          {renderMiniBoard("A")}
        </div>
        <div className="text-xl font-bold text-line select-none">↔</div>
        <div className="flex flex-col items-center gap-1.5">
          <span className="text-xs font-semibold text-moss uppercase tracking-wider">{labelB}</span>
          {renderMiniBoard("B")}
        </div>
      </div>
      
      <div className="flex justify-center gap-4 text-xs text-moss">
        <div className="flex items-center gap-1">
          <span className="w-3.5 h-3.5 bg-red-200/60 border border-red-300 rounded" />
          <span>Captured / Missing</span>
        </div>
        <div className="flex items-center gap-1">
          <span className="w-3.5 h-3.5 bg-emerald-200/60 border border-emerald-300 rounded" />
          <span>Added / Advanced</span>
        </div>
        <div className="flex items-center gap-1">
          <span className="w-3.5 h-3.5 bg-amber-100/60 border border-amber-200 rounded" />
          <span>Changed / Moved</span>
        </div>
      </div>
    </div>
  );
}
