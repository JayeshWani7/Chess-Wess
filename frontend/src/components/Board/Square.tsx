import type { Piece } from "chess.js";
import PieceIcon from "./PieceIcon";
import { motion } from "framer-motion";

interface SquareProps {
  square: string;
  piece: Piece | null;
  isLight: boolean;
  isSelected: boolean;
  isLegal: boolean;
  isLastMove: boolean;
  onClick: (square: string) => void;
}

export default function Square({
  square,
  piece,
  isLight,
  isSelected,
  isLegal,
  isLastMove,
  onClick,
}: SquareProps) {
  let bg = isLight ? "bg-board-light" : "bg-board-dark";
  if (isSelected) bg = "bg-board-selected";
  else if (isLastMove) bg = "bg-board-lastmove";

  return (
    <div
      role="gridcell"
      aria-label={square}
      className={`relative flex items-center justify-center cursor-pointer ${bg} transition-colors hover:brightness-105`}
      style={{ aspectRatio: "1 / 1", minWidth: 0, minHeight: 0 }}
      onClick={() => onClick(square)}
    >
      {isLegal && !piece && (
        <div className="absolute w-1/3 h-1/3 rounded-full bg-ink/20 pointer-events-none" />
      )}
      {isLegal && piece && (
        <div className="absolute inset-0 rounded-sm ring-4 ring-inset ring-ink/30 pointer-events-none" />
      )}

      {piece && (
        <motion.div
          key={`${piece.color}${piece.type}-${square}`}
          initial={{ scale: 0.85 }}
          animate={{ scale: 1 }}
          className="absolute inset-0 flex items-center justify-center"
        >
          <PieceIcon piece={piece} />
        </motion.div>
      )}
    </div>
  );
}
