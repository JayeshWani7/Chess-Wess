import { useEffect, useMemo, useState } from "react";
import type { TimelineData, TimelineNode } from "../../store/gameStore";
import TimelineGraph from "./TimelineGraph";

interface TimelinePanelProps {
  timelines: TimelineData[];
  activeTimelineId: string | null;
  activeTimelineLatestNodeId: string | null;
  selectedNodeId: string | null;
  nodesById: Record<string, TimelineNode>;
  onSelectNode: (nodeId: string) => void;
  onRewind: (nodeId: string) => void;
  onSwitchTimeline: (timelineId: string) => void;
  onRenameTimeline: (timelineId: string, name: string) => void;
  onLoadMoreGraph: () => void;
  onLoadFullGraph: () => void;
  nodeLimit: number | null;
  merges?: { id: string; game_id: string; source_node_id: string; target_node_id: string }[];
  sandboxMoves?: TimelineNode[];
}

function shortId(id: string) {
  return id ? id.slice(0, 8) : "";
}

type MaterialBreakdown = {
  white: number;
  black: number;
  diff: number;
};

type PieceCounts = {
  white: Record<string, number>;
  black: Record<string, number>;
};

function computeMaterial(fen: string): MaterialBreakdown {
  const pieceValues: Record<string, number> = {
    p: 1,
    n: 3,
    b: 3,
    r: 5,
    q: 9,
    k: 0,
  };
  const board = fen.split(" ")[0] ?? "";
  let white = 0;
  let black = 0;

  for (const char of board) {
    if (char === "/") continue;
    if (char >= "1" && char <= "8") continue;

    const value = pieceValues[char.toLowerCase()] ?? 0;
    if (char === char.toUpperCase()) white += value;
    else black += value;
  }

  return { white, black, diff: white - black };
}

function computePieceCounts(fen: string): PieceCounts {
  const board = fen.split(" ")[0] ?? "";
  const emptyCounts = { p: 0, n: 0, b: 0, r: 0, q: 0, k: 0 };
  const white = { ...emptyCounts };
  const black = { ...emptyCounts };

  for (const char of board) {
    if (char === "/") continue;
    if (char >= "1" && char <= "8") continue;

    const key = char.toLowerCase() as keyof typeof emptyCounts;
    if (char === char.toUpperCase()) white[key] += 1;
    else black[key] += 1;
  }

  return { white, black };
}

export default function TimelinePanel({
  timelines,
  activeTimelineId,
  activeTimelineLatestNodeId,
  selectedNodeId,
  nodesById,
  onSelectNode,
  onRewind,
  onSwitchTimeline,
  onRenameTimeline,
  onLoadMoreGraph,
  onLoadFullGraph,
  nodeLimit,
  merges,
  sandboxMoves,
}: TimelinePanelProps) {
  const activeTimeline = useMemo(
    () => timelines.find((t) => t.timeline_id === activeTimelineId) ?? null,
    [timelines, activeTimelineId]
  );
  const selectedNode = selectedNodeId ? nodesById[selectedNodeId] : null;
  const activeNode = activeTimelineLatestNodeId
    ? nodesById[activeTimelineLatestNodeId]
    : null;
  const statsNode = selectedNode ?? activeNode;
  const material = statsNode ? computeMaterial(statsNode.board_state) : null;
  const counts = statsNode ? computePieceCounts(statsNode.board_state) : null;

  const [nameDraft, setNameDraft] = useState("");
  useEffect(() => {
    const fallback = activeTimelineId ? `Timeline ${shortId(activeTimelineId)}` : "";
    setNameDraft(activeTimeline?.timeline_name ?? fallback);
  }, [activeTimeline?.timeline_name, activeTimelineId]);

  const rewindTargetId = selectedNodeId ?? activeTimelineLatestNodeId;

  return (
    <div className="card w-full">
      <div className="flex flex-col gap-4">
        <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
          <div>
            <h3 className="text-sm font-semibold text-ink">Timeline Graph</h3>
            <p className="text-xs text-moss">
              Active timeline: {activeTimeline?.timeline_name ?? (activeTimelineId ? shortId(activeTimelineId) : "none")}
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <select
              className="input text-xs max-w-[220px]"
              value={activeTimelineId ?? ""}
              onChange={(e) => onSwitchTimeline(e.target.value)}
              disabled={timelines.length === 0}
            >
              {timelines.length === 0 && <option value="">No timelines</option>}
              {timelines.map((t) => (
                <option key={t.timeline_id} value={t.timeline_id}>
                  {t.timeline_name ?? `Timeline ${shortId(t.timeline_id)}`}
                </option>
              ))}
            </select>
            <button
              className="btn-outline text-xs"
              disabled={!rewindTargetId}
              onClick={() => rewindTargetId && onRewind(rewindTargetId)}
            >
              Rewind & Branch
            </button>
          </div>
        </div>

        <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
          <div className="flex items-center gap-2">
            <input
              className="input text-xs max-w-[220px]"
              placeholder="Name this timeline"
              value={nameDraft}
              onChange={(e) => setNameDraft(e.target.value)}
              disabled={!activeTimelineId}
              onKeyDown={(e) => {
                if (e.key === "Enter" && activeTimelineId) {
                  onRenameTimeline(activeTimelineId, nameDraft);
                }
              }}
            />
            <button
              className="btn-outline text-xs"
              disabled={!activeTimelineId || !nameDraft.trim()}
              onClick={() => activeTimelineId && onRenameTimeline(activeTimelineId, nameDraft)}
            >
              Save Name
            </button>
          </div>
          {activeTimeline?.nodes_partial && (
            <div className="flex items-center gap-2 text-xs text-moss">
              <span>
                Showing last {nodeLimit ?? 0} of {activeTimeline.node_count ?? "?"} nodes
              </span>
              <button className="btn-ghost text-xs" onClick={onLoadMoreGraph}>
                Load more
              </button>
              <button className="btn-ghost text-xs" onClick={onLoadFullGraph}>
                Load full
              </button>
            </div>
          )}
        </div>

        <TimelineGraph
          timelines={timelines}
          activeTimelineId={activeTimelineId}
          selectedNodeId={selectedNodeId}
          onSelectNode={onSelectNode}
          merges={merges}
          sandboxMoves={sandboxMoves}
        />

        <div className="grid gap-3 md:grid-cols-3">
          <div className="rounded-lg border border-line bg-paper p-3">
            <p className="text-xs uppercase text-moss">Active Node</p>
            {activeNode ? (
              <div className="mt-2 text-xs text-ink space-y-1">
                <p>Node: {shortId(activeNode.id)}</p>
                <p>Turn: {activeNode.turn_number}</p>
                <p>Move: {activeNode.move?.san ?? "Root"}</p>
              </div>
            ) : (
              <p className="mt-2 text-xs text-moss">No active node</p>
            )}
          </div>

          <div className="rounded-lg border border-line bg-paper p-3">
            <p className="text-xs uppercase text-moss">Inspector</p>
            {selectedNode ? (
              <div className="mt-2 text-xs text-ink space-y-1">
                <p>Node: {shortId(selectedNode.id)}</p>
                <p>Timeline: {shortId(selectedNode.timeline_id)}</p>
                <p>Turn: {selectedNode.turn_number}</p>
                <p>Move: {selectedNode.move?.san ?? "Root"}</p>
                <p className="text-moss">FEN: {selectedNode.board_state}</p>
              </div>
            ) : (
              <p className="mt-2 text-xs text-moss">Select a node to inspect</p>
            )}
          </div>

          <div className="rounded-lg border border-line bg-paper p-3">
            <p className="text-xs uppercase text-moss">Timeline Stats</p>
            {statsNode && material && counts ? (
              <div className="mt-2 text-xs text-ink space-y-1">
                <p>
                  Material: W {material.white} / B {material.black}
                </p>
                <p>
                  Pieces W: P{counts.white.p} N{counts.white.n} B{counts.white.b} R{counts.white.r} Q{counts.white.q}
                </p>
                <p>
                  Pieces B: P{counts.black.p} N{counts.black.n} B{counts.black.b} R{counts.black.r} Q{counts.black.q}
                </p>
                <p>
                  Advantage: {material.diff === 0 ? "Even" : material.diff > 0 ? `+${material.diff}` : material.diff}
                </p>
                <p>
                  Eval: {(typeof statsNode.metadata?.evaluation === "number"
                    ? statsNode.metadata.evaluation
                    : 0
                  ).toFixed(2)}
                </p>
                <p className="text-moss">
                  Based on {selectedNode ? "selected" : "active"} node
                </p>
              </div>
            ) : (
              <p className="mt-2 text-xs text-moss">No stats available</p>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
