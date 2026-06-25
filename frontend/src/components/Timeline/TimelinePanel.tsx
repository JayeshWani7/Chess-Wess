import { useEffect, useMemo, useState } from "react";
import type { TimelineData, TimelineNode } from "../../store/gameStore";
import { useGameStore } from "../../store/gameStore";
import { useAuthStore } from "../../store/authStore";
import { api } from "../../utils/api";
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
  const annotations = useGameStore((s) => s.annotations);
  const selectedCompareNodeId = useGameStore((s) => s.selectedCompareNodeId);
  const selectCompareNode = useGameStore((s) => s.selectCompareNode);
  const addAnnotationLocal = useGameStore((s) => s.addAnnotationLocal);
  const activeGameId = useGameStore((s) => s.activeGameId);
  const timelineMetadata = useGameStore((s) => s.timelineMetadata);
  const { token, userId, username } = useAuthStore();

  const [searchQuery, setSearchQuery] = useState("");
  const [filterLocked, setFilterLocked] = useState(false);
  const [filterCollapsed, setFilterCollapsed] = useState(false);

  const [annDraft, setAnnDraft] = useState("");
  const [tagDraft, setTagDraft] = useState("");

  useEffect(() => {
    setAnnDraft("");
    setTagDraft("");
  }, [selectedNodeId]);

  const handleAddAnnotation = async () => {
    if (!token || !activeGameId || !selectedNodeId || !annDraft.trim()) return;
    try {
      await api.annotateNode(token, activeGameId, selectedNodeId, annDraft.trim(), tagDraft);
      addAnnotationLocal(selectedNodeId, userId!, username!, annDraft.trim(), tagDraft || null);
      setAnnDraft("");
      setTagDraft("");
    } catch (err) {
      console.error("Failed to add annotation:", err);
    }
  };

  const highlightedNodeIds = useMemo(() => {
    const set = new Set<string>();
    const query = searchQuery.trim().toLowerCase();
    if (!query) return set;

    for (const t of timelines) {
      for (const n of t.nodes) {
        const moveSan = n.move?.san?.toLowerCase() ?? "";
        const moveUci = n.move?.uci?.toLowerCase() ?? "";
        const fen = n.board_state.toLowerCase();
        
        if (moveSan.includes(query) || moveUci.includes(query) || fen.includes(query)) {
          set.add(n.id);
        }
      }
    }
    return set;
  }, [timelines, searchQuery]);

  const filteredTimelines = useMemo(() => {
    return timelines.filter((t) => {
      const metadata = timelineMetadata[t.timeline_id];
      if (filterLocked && metadata?.is_locked) return false;
      if (filterCollapsed && metadata?.is_collapsed) return false;
      return true;
    });
  }, [timelines, timelineMetadata, filterLocked, filterCollapsed]);

  const breadcrumbs = useMemo(() => {
    const path: TimelineNode[] = [];
    let currId: string | null = selectedNodeId || activeTimelineLatestNodeId;
    while (currId && nodesById[currId]) {
      const node = nodesById[currId];
      path.push(node);
      currId = node.parent_node_id;
    }
    return path.reverse();
  }, [selectedNodeId, activeTimelineLatestNodeId, nodesById]);

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

        {/* Search & Filter HUD */}
        <div className="flex flex-col gap-2.5 md:flex-row md:items-center">
          <div className="flex items-center gap-2 flex-1">
            <input
              type="text"
              placeholder="Search nodes (e.g. Nf3, Qxh7, FEN)..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="input text-xs flex-1"
            />
            {searchQuery && (
              <button
                onClick={() => setSearchQuery("")}
                className="text-xs text-moss hover:text-ink"
              >
                Clear
              </button>
            )}
          </div>
          <div className="flex flex-wrap items-center gap-3 text-xs text-moss">
            <label className="flex items-center gap-1.5 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={filterLocked}
                onChange={(e) => setFilterLocked(e.target.checked)}
                className="rounded border-line text-purple-600 focus:ring-purple-500"
              />
              <span>Hide Locked</span>
            </label>
            <label className="flex items-center gap-1.5 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={filterCollapsed}
                onChange={(e) => setFilterCollapsed(e.target.checked)}
                className="rounded border-line text-purple-600 focus:ring-purple-500"
              />
              <span>Hide Collapsed</span>
            </label>
          </div>
        </div>

        {/* Breadcrumb Path Bar */}
        {breadcrumbs.length > 0 && (
          <div className="flex flex-wrap items-center gap-1.5 bg-paper rounded-lg p-2 border border-line text-xs font-mono select-none">
            <span className="text-moss font-semibold uppercase tracking-wider mr-1">Path:</span>
            {breadcrumbs.map((node, idx) => {
              const moveText = node.move?.san ?? (node.turn_number === 0 ? "Root" : `T${node.turn_number}`);
              const isLast = idx === breadcrumbs.length - 1;
              const parentBranches = timelines.flatMap(t => t.nodes).filter(n => n.parent_node_id === node.id);
              const hasDivergence = parentBranches.length > 1;

              return (
                <div key={node.id} className="flex items-center gap-1">
                  {idx > 0 && <span className="text-moss">›</span>}
                  <button
                    onClick={() => onSelectNode(node.id)}
                    className={`hover:underline rounded px-1 py-0.5 ${
                      isLast
                        ? "text-pine font-bold bg-mist"
                        : "text-ink"
                    }`}
                  >
                    {moveText}
                  </button>
                  {hasDivergence && (
                    <span 
                      className="inline-flex items-center px-1.5 py-0.2 rounded-full text-[9px] font-semibold bg-purple-100 text-purple-700 cursor-pointer"
                      title={`${parentBranches.length} branch pathways fork from here`}
                      onClick={() => onSelectNode(node.id)}
                    >
                      +{parentBranches.length - 1} split
                    </span>
                  )}
                </div>
              );
            })}
          </div>
        )}

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
          timelines={filteredTimelines}
          activeTimelineId={activeTimelineId}
          selectedNodeId={selectedNodeId}
          onSelectNode={onSelectNode}
          merges={merges}
          sandboxMoves={sandboxMoves}
          highlightedNodeIds={highlightedNodeIds}
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

          <div className="rounded-lg border border-line bg-paper p-3 flex flex-col justify-between">
            <div>
              <p className="text-xs uppercase text-moss">Inspector</p>
              {selectedNode ? (
                <div className="mt-2 text-xs text-ink space-y-1.5">
                  <p><span className="font-semibold">Node:</span> {shortId(selectedNode.id)}</p>
                  <p><span className="font-semibold">Timeline:</span> {shortId(selectedNode.timeline_id)}</p>
                  <p><span className="font-semibold">Turn:</span> {selectedNode.turn_number}</p>
                  <p><span className="font-semibold">Move:</span> {selectedNode.move?.san ?? "Root"}</p>
                  <p className="text-moss text-[10px] break-all"><span className="font-semibold">FEN:</span> {selectedNode.board_state}</p>
                  
                  {/* Annotation List */}
                  {annotations[selectedNode.id]?.length > 0 && (
                    <div className="mt-2 pt-2 border-t border-line space-y-1 max-h-[140px] overflow-y-auto">
                      <p className="text-[9px] uppercase font-bold text-purple-600">Annotations:</p>
                      {annotations[selectedNode.id].map((ann, idx) => (
                        <div key={idx} className="bg-panel rounded p-1.5 border border-line text-[10px]">
                          <div className="flex items-center justify-between mb-0.5 font-sans">
                            <span className="font-semibold text-ink">{ann.username}</span>
                            {ann.label_tag && (
                              <span className="px-1 py-0.2 rounded text-[9px] font-bold bg-purple-100 text-purple-700 capitalize">
                                {ann.label_tag}
                              </span>
                            )}
                          </div>
                          <p className="text-moss italic">"{ann.annotation}"</p>
                        </div>
                      ))}
                    </div>
                  )}

                  {/* Add Annotation Form */}
                  <div className="mt-2 pt-2 border-t border-line space-y-1.5">
                    <textarea
                      value={annDraft}
                      onChange={(e) => setAnnDraft(e.target.value)}
                      placeholder="Add strategic comment..."
                      className="input text-[10px] w-full h-11 resize-none p-1"
                    />
                    <div className="flex gap-1.5">
                      <select
                        value={tagDraft}
                        onChange={(e) => setTagDraft(e.target.value)}
                        className="input text-[10px] py-0.5 px-1 flex-1"
                      >
                        <option value="">No tag</option>
                        <option value="blunder">Blunder</option>
                        <option value="brilliant">Brilliant</option>
                        <option value="critical">Critical</option>
                        <option value="theoretical">Theoretical</option>
                      </select>
                      <button
                        onClick={handleAddAnnotation}
                        disabled={!annDraft.trim()}
                        className="btn bg-purple-600 hover:bg-purple-700 text-white text-[10px] py-0.5 px-2 rounded transition-colors disabled:opacity-40"
                      >
                        Comment
                      </button>
                    </div>
                  </div>
                </div>
              ) : (
                <p className="mt-2 text-xs text-moss">Select a node to inspect</p>
              )}
            </div>

            {selectedNode && (
              <div className="mt-2 pt-2 border-t border-line flex flex-col">
                {selectedCompareNodeId !== selectedNode.id ? (
                  <button
                    onClick={() => selectCompareNode(selectedNode.id)}
                    className="btn-outline text-[10px] py-1 text-center"
                  >
                    Compare This Position
                  </button>
                ) : (
                  <button
                    onClick={() => selectCompareNode(null)}
                    className="btn bg-amber-500 hover:bg-amber-600 text-white text-[10px] py-1 text-center rounded transition-colors"
                  >
                    Comparing... (Click to cancel)
                  </button>
                )}
              </div>
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
