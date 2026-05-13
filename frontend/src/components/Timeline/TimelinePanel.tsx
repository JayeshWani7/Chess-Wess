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
}

function shortId(id: string) {
  return id ? id.slice(0, 8) : "";
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
}: TimelinePanelProps) {
  const selectedNode = selectedNodeId ? nodesById[selectedNodeId] : null;
  const activeNode = activeTimelineLatestNodeId
    ? nodesById[activeTimelineLatestNodeId]
    : null;

  const rewindTargetId = selectedNodeId ?? activeTimelineLatestNodeId;

  return (
    <div className="card w-full">
      <div className="flex flex-col gap-4">
        <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
          <div>
            <h3 className="text-sm font-semibold text-gray-300">Timeline Graph</h3>
            <p className="text-xs text-gray-500">
              Active timeline: {activeTimelineId ? shortId(activeTimelineId) : "none"}
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
                  Timeline {shortId(t.timeline_id)}
                </option>
              ))}
            </select>
            <button
              className="btn-ghost text-xs"
              disabled={!rewindTargetId}
              onClick={() => rewindTargetId && onRewind(rewindTargetId)}
            >
              Rewind & Branch
            </button>
          </div>
        </div>

        <TimelineGraph
          timelines={timelines}
          activeTimelineId={activeTimelineId}
          selectedNodeId={selectedNodeId}
          onSelectNode={onSelectNode}
        />

        <div className="grid gap-3 md:grid-cols-2">
          <div className="rounded-lg border border-chrono-border bg-chrono-bg/60 p-3">
            <p className="text-xs uppercase text-gray-500">Active Node</p>
            {activeNode ? (
              <div className="mt-2 text-xs text-gray-300 space-y-1">
                <p>Node: {shortId(activeNode.id)}</p>
                <p>Turn: {activeNode.turn_number}</p>
                <p>Move: {activeNode.move?.san ?? "Root"}</p>
              </div>
            ) : (
              <p className="mt-2 text-xs text-gray-500">No active node</p>
            )}
          </div>

          <div className="rounded-lg border border-chrono-border bg-chrono-bg/60 p-3">
            <p className="text-xs uppercase text-gray-500">Inspector</p>
            {selectedNode ? (
              <div className="mt-2 text-xs text-gray-300 space-y-1">
                <p>Node: {shortId(selectedNode.id)}</p>
                <p>Timeline: {shortId(selectedNode.timeline_id)}</p>
                <p>Turn: {selectedNode.turn_number}</p>
                <p>Move: {selectedNode.move?.san ?? "Root"}</p>
                <p className="text-gray-500">FEN: {selectedNode.board_state}</p>
              </div>
            ) : (
              <p className="mt-2 text-xs text-gray-500">Select a node to inspect</p>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
