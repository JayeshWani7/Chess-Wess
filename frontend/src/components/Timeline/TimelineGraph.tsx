import { useMemo } from "react";
import ReactFlow, {
  Background,
  Controls,
  type Edge,
  type Node,
  type NodeMouseHandler,
} from "reactflow";
import "reactflow/dist/style.css";
import type { TimelineData, TimelineNode } from "../../store/gameStore";

interface TimelineGraphProps {
  timelines: TimelineData[];
  activeTimelineId: string | null;
  selectedNodeId: string | null;
  onSelectNode: (nodeId: string) => void;
}

const NODE_X_STEP = 160;
const NODE_Y_STEP = 120;

function buildBranchEdges(nodes: TimelineNode[], nodeIds: Set<string>): Edge[] {
  const edges: Edge[] = [];
  const byKey = new Map<string, TimelineNode[]>();

  for (const node of nodes) {
    const key = `${node.turn_number}-${node.board_state}`;
    const list = byKey.get(key) ?? [];
    list.push(node);
    byKey.set(key, list);
  }

  for (const node of nodes) {
    if (node.parent_node_id || node.turn_number === 0) continue;
    const key = `${node.turn_number}-${node.board_state}`;
    const candidates = (byKey.get(key) ?? []).filter((n) => n.id !== node.id);
    if (!candidates.length) continue;

    candidates.sort((a, b) => a.created_at.localeCompare(b.created_at));
    const parent = candidates[0];
    if (!nodeIds.has(parent.id) || !nodeIds.has(node.id)) continue;

    edges.push({
      id: `branch-${parent.id}-${node.id}`,
      source: parent.id,
      target: node.id,
      type: "smoothstep",
      style: { strokeDasharray: "4 6", stroke: "#64748b" },
    });
  }

  return edges;
}

export default function TimelineGraph({
  timelines,
  activeTimelineId,
  selectedNodeId,
  onSelectNode,
}: TimelineGraphProps) {
  const handleNodeClick: NodeMouseHandler = (_, node) => onSelectNode(node.id);

  const { nodes, edges } = useMemo(() => {
    const rfNodes: Node[] = [];
    const rfEdges: Edge[] = [];
    const timelineIndex = new Map<string, number>();
    const nodeIds = new Set<string>();

    timelines.forEach((t, idx) => timelineIndex.set(t.timeline_id, idx));

    const allNodes: TimelineNode[] = [];
    for (const timeline of timelines) {
      for (const node of timeline.nodes) {
        allNodes.push(node);
        nodeIds.add(node.id);
      }
    }

    for (const node of allNodes) {
      const x = node.turn_number * NODE_X_STEP;
      const y = (timelineIndex.get(node.timeline_id) ?? 0) * NODE_Y_STEP;

      rfNodes.push({
        id: node.id,
        position: { x, y },
        data: {
          label: node.move?.san ?? (node.turn_number === 0 ? "Root" : `T${node.turn_number}`),
        },
        style: {
          background: node.timeline_id === activeTimelineId ? "#1f2937" : "#111827",
          border:
            node.id === selectedNodeId
              ? "2px solid #38bdf8"
              : node.timeline_id === activeTimelineId
              ? "1px solid #22c55e"
              : "1px solid #334155",
          color: "#e2e8f0",
          padding: "6px 10px",
          borderRadius: 10,
          fontSize: 12,
        },
      });

      if (node.parent_node_id && nodeIds.has(node.parent_node_id)) {
        rfEdges.push({
          id: `edge-${node.parent_node_id}-${node.id}`,
          source: node.parent_node_id,
          target: node.id,
          type: "smoothstep",
          style: { stroke: "#475569" },
        });
      }
    }

    return {
      nodes: rfNodes,
      edges: [...rfEdges, ...buildBranchEdges(allNodes, nodeIds)],
    };
  }, [timelines, activeTimelineId, selectedNodeId]);

  return (
    <div className="h-[360px] w-full rounded-xl border border-chrono-border bg-chrono-surface">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        fitView
        nodesDraggable={false}
        nodesConnectable={false}
        onNodeClick={handleNodeClick}
      >
        <Background gap={24} color="#1f2937" />
        <Controls position="top-right" />
      </ReactFlow>
    </div>
  );
}
