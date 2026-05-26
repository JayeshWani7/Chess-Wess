import { motion } from "framer-motion";

interface RulesModalProps {
  onClose: () => void;
}

export default function RulesModal({ onClose }: RulesModalProps) {
  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      className="fixed inset-0 z-50 flex items-center justify-center bg-ink/70 p-4"
      role="dialog"
      aria-modal="true"
      aria-label="ChessWess rules"
    >
      <motion.div
        initial={{ scale: 0.9, y: 18 }}
        animate={{ scale: 1, y: 0 }}
        className="card w-full max-w-xl space-y-4"
      >
        <header className="space-y-1">
          <p className="text-xs uppercase tracking-[0.3em] text-moss">How to Play</p>
          <h2 className="text-2xl font-display text-ink">Timeline Rules</h2>
          <p className="text-sm text-moss">
            Every move creates a new node in the timeline. Rewinding creates a
            branch.
          </p>
        </header>

        <div className="space-y-3 text-sm text-ink/80">
          <div className="rounded-xl border border-line/70 bg-paper/60 p-3">
            <p className="font-semibold text-ink">Core Rules</p>
            <ul className="mt-2 list-disc space-y-1 pl-5">
              <li>Win by checkmate, resignation, or time.</li>
              <li>Click any previous node to rewind and create a new branch.</li>
              <li>The active timeline is the one you are currently playing.</li>
            </ul>
          </div>

          <div className="rounded-xl border border-line/70 bg-paper/60 p-3">
            <p className="font-semibold text-ink">Energy System</p>
            <ul className="mt-2 list-disc space-y-1 pl-5">
              <li>Rewinding costs energy based on how many turns you go back.</li>
              <li>Switching timelines costs energy (see the Energy panel).</li>
              <li>If you are out of energy, rewinds and jumps are blocked.</li>
            </ul>
          </div>

          <div className="rounded-xl border border-line/70 bg-paper/60 p-3">
            <p className="font-semibold text-ink">Tips</p>
            <ul className="mt-2 list-disc space-y-1 pl-5">
              <li>Use the timeline panel to inspect branches before switching.</li>
              <li>Save energy for critical rewinds in the endgame.</li>
            </ul>
          </div>
        </div>

        <div className="flex justify-end gap-2 pt-2">
          <button onClick={onClose} className="btn-primary">
            Got it, start game
          </button>
        </div>
      </motion.div>
    </motion.div>
  );
}
