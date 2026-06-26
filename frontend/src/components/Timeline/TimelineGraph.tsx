import { useMemo, useState, useRef } from "react";
import dagre from "dagre";
import ReactFlow, {
  Background,
  Controls,
  MiniMap,
  type Edge,
  type Node,
  type NodeMouseHandler,
} from "reactflow";
import "reactflow/dist/style.css";
import type { TimelineData, TimelineNode } from "../../store/gameStore";
import { applyRenderBudget, MAX_RENDER_NODES } from "../../store/timelineMemory";

interface TimelineGraphProps {
  timelines: TimelineData[];
  activeTimelineId: string | null;
  selectedNodeId: string | null;
  onSelectNode: (nodeId: string) => void;
  merges?: { source_node_id: string; target_node_id: string }[];
  sandboxMoves?: TimelineNode[];
  highlightedNodeIds?: Set<string>;
}

const NODE_X_STEP = 160;
const NODE_Y_STEP = 120;
const NODE_WIDTH = 132;
const NODE_HEIGHT = 48;
const EVAL_CLAMP = 6;

interface MiniBoardProps {
  fen: string;
}

function MiniBoard({ fen }: MiniBoardProps) {
  const boardPart = fen.split(" ")[0] ?? "";
  const rows = boardPart.split("/");
  
  const cells: { key: string; symbol: string; color: "w" | "b"; isLight: boolean }[] = [];
  
  const PIECES: Record<string, string> = {
    k: "♚", q: "♛", r: "♜", b: "♝", n: "♞", p: "♟",
    K: "♔", Q: "♕", R: "♖", B: "♗", N: "♘", P: "♙",
  };

  for (let r = 0; r < 8; r++) {
    const rowStr = rows[r] ?? "";
    let col = 0;
    for (let i = 0; i < rowStr.length; i++) {
      const char = rowStr[i];
      if (char >= "1" && char <= "8") {
        const emptyCount = parseInt(char, 10);
        for (let e = 0; e < emptyCount; e++) {
          cells.push({
            key: `cell-${r}-${col}`,
            symbol: "",
            color: "w",
            isLight: (r + col) % 2 === 0
          });
          col++;
        }
      } else {
        const isWhite = char === char.toUpperCase();
        cells.push({
          key: `cell-${r}-${col}`,
          symbol: PIECES[char] ?? char,
          color: isWhite ? "w" : "b",
          isLight: (r + col) % 2 === 0
        });
        col++;
      }
    }
  }
  
  return (
    <div 
      className="grid grid-cols-8 gap-0 border border-line" 
      style={{ width: "96px", height: "96px" }}
    >
      {cells.map((cell) => (
        <div
          key={cell.key}
          className={`flex items-center justify-center ${cell.isLight ? "bg-[#f2e8d5]" : "bg-[#b2a991]"}`}
          style={{ width: "12px", height: "12px" }}
        >
          {cell.symbol && (
            <span
              className="text-[10px] leading-none select-none font-bold"
              style={{
                color: cell.color === "w" ? "#fff" : "#1b1e1a",
                textShadow: cell.color === "w" ? "0 0 1px #000" : "none",
              }}
            >
              {cell.symbol}
            </span>
          )}
        </div>
      ))}
    </div>
  );
}

// Shared dagre graph instance — re-used across renders (reset each layout call)
const dagreGraph = new dagre.graphlib.Graph();
dagreGraph.setDefaultEdgeLabel(() => ({}));

function evaluationColor(score?: number | null) {
  if (typeof score !== "number") return "#c8c1b2";
  const clamped = Math.max(-EVAL_CLAMP, Math.min(EVAL_CLAMP, score));
  const normalized = (clamped + EVAL_CLAMP) / (EVAL_CLAMP * 2);
  const hue = 120 * normalized;
  return `hsl(${hue} 55% 48%)`;
}

function layoutDag(nodes: Node[], edges: Edge[]) {
  // Clear previous layout data
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  dagreGraph.nodes().forEach((n: any) => dagreGraph.removeNode(n));

  dagreGraph.setGraph({
    rankdir: "LR",
    ranksep: NODE_X_STEP,
    nodesep: NODE_Y_STEP,
  });

  nodes.forEach((node) => {
    dagreGraph.setNode(node.id, { width: NODE_WIDTH, height: NODE_HEIGHT });
  });

  edges.forEach((edge) => {
    // Guard: dagre requires both endpoints to exist
    if (dagreGraph.hasNode(edge.source) && dagreGraph.hasNode(edge.target)) {
      dagreGraph.setEdge(edge.source, edge.target);
    }
  });

  dagre.layout(dagreGraph);

  const layoutedNodes = nodes.map((node) => {
    const layoutNode = dagreGraph.node(node.id);
    if (!layoutNode) return node;
    return {
      ...node,
      position: {
        x: layoutNode.x - NODE_WIDTH / 2,
        y: layoutNode.y - NODE_HEIGHT / 2,
      },
    };
  });

  return { nodes: layoutedNodes, edges };
}

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
      style: { strokeDasharray: "4 6", stroke: "#8a8f85" },
    });
  }

  return edges;
}

export default function TimelineGraph({
  timelines,
  activeTimelineId,
  selectedNodeId,
  onSelectNode,
  merges,
  sandboxMoves,
  highlightedNodeIds,
}: TimelineGraphProps) {
  const handleNodeClick: NodeMouseHandler = (_, node) => onSelectNode(node.id);

  const [hoveredNode, setHoveredNode] = useState<{ label: string; fen: string; x: number; y: number } | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  const handleNodeMouseEnter = (event: React.MouseEvent, node: Node) => {
    const container = containerRef.current;
    if (!container) return;
    const rect = container.getBoundingClientRect();
    
    const allTimelineNodes = [
      ...timelines.flatMap((t) => t.nodes),
      ...(sandboxMoves ?? []),
    ];
    const matching = allTimelineNodes.find((n) => n.id === node.id);
    
    if (matching) {
      setHoveredNode({
        label: matching.move?.san ?? (matching.turn_number === 0 ? "Root" : `T${matching.turn_number}`),
        fen: matching.board_state,
        x: event.clientX - rect.left,
        y: event.clientY - rect.top,
      });
    }
  };

  const handleNodeMouseLeave = () => {
    setHoveredNode(null);
  };

  const { nodes, edges } = useMemo(() => {
    const timelinesWithSandbox = [...timelines];
    if (sandboxMoves && sandboxMoves.length > 0) {
      timelinesWithSandbox.push({
        timeline_id: "sandbox",
        timeline_name: "Sandbox",
        nodes: sandboxMoves,
        node_count: sandboxMoves.length,
      });
    }

    // ── Apply render budget before building React Flow nodes ──────────────
    const budgeted = applyRenderBudget({
      timelines: timelinesWithSandbox,
      activeTimelineId,
      selectedNodeId,
      maxNodes: MAX_RENDER_NODES,
    });

    const rfNodes: Node[] = [];
    const rfEdges: Edge[] = [];
    const timelineIndex = new Map<string, number>();
    const nodeIds = new Set<string>();

    budgeted.forEach((t, idx) => timelineIndex.set(t.timeline_id, idx));

    const allNodes: TimelineNode[] = [];
    for (const timeline of budgeted) {
      for (const node of timeline.nodes) {
        allNodes.push(node);
        nodeIds.add(node.id);
      }
    }

    for (const node of allNodes) {
      const x = node.turn_number * NODE_X_STEP;
      const y = (timelineIndex.get(node.timeline_id) ?? 0) * NODE_Y_STEP;
      const evalColor = evaluationColor(node.metadata?.evaluation ?? null);
      const isSandbox = node.timeline_id === "sandbox";
      const isActive = node.timeline_id === activeTimelineId;
      const isSelected = node.id === selectedNodeId;
      const isHighlighted = highlightedNodeIds?.has(node.id) ?? false;

      rfNodes.push({
        id: node.id,
        position: { x, y },
        data: {
          label: node.move?.san ?? (node.turn_number === 0 ? "Root" : `T${node.turn_number}`),
          evaluation: node.metadata?.evaluation ?? null,
        },
        style: {
          background: isSandbox ? "#fef08a" : (isActive ? "#f2e8d5" : "#fcf8f1"),
          border: isSelected
            ? "3px solid #c9a227"
            : isHighlighted
            ? "3px solid #a855f7"
            : isSandbox
            ? "1px dashed #eab308"
            : isActive
            ? "1px solid #4b7a2c"
            : `1px solid ${evalColor}`,
          boxShadow: isSelected
            ? `0 0 0 3px ${evalColor}40`
            : isHighlighted
            ? "0 0 8px 2px rgba(168, 85, 247, 0.4)"
            : "none",
          color: "#1b1e1a",
          padding: "6px 10px",
          borderRadius: 10,
          fontSize: 12,
          width: NODE_WIDTH,
          height: NODE_HEIGHT,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          textAlign: "center" as const,
        },
      });

      if (node.parent_node_id && nodeIds.has(node.parent_node_id)) {
        rfEdges.push({
          id: `edge-${node.parent_node_id}-${node.id}`,
          source: node.parent_node_id,
          target: node.id,
          type: "smoothstep",
          style: { stroke: isSandbox ? "#eab308" : "#b2a991", strokeDasharray: isSandbox ? "3 3" : "none" },
        });
      }
    }

    // Add merge edges
    if (merges) {
      for (const m of merges) {
        if (nodeIds.has(m.source_node_id) && nodeIds.has(m.target_node_id)) {
          rfEdges.push({
            id: `merge-${m.source_node_id}-${m.target_node_id}`,
            source: m.source_node_id,
            target: m.target_node_id,
            type: "smoothstep",
            animated: true,
            label: "Merge",
            style: { stroke: "#a855f7", strokeWidth: 2, strokeDasharray: "5 5" },
            labelStyle: { fill: "#a855f7", fontWeight: 700, fontSize: 10 },
          });
        }
      }
    }

    return layoutDag(rfNodes, [...rfEdges, ...buildBranchEdges(allNodes, nodeIds)]);
  }, [timelines, activeTimelineId, selectedNodeId, sandboxMoves, merges]);

  // Node count badge for transparency when budget is applied
  const totalNodes = timelines.reduce((s, t) => s + t.nodes.length, 0);
  const renderingAll = nodes.length >= totalNodes;

  return (
    <div ref={containerRef} className="relative h-[360px] w-full rounded-xl border border-line bg-panel">
      {/* Render budget indicator */}
      {!renderingAll && (
        <div className="absolute top-2 left-2 z-10 text-xs bg-black/50 text-amber-300 px-2 py-1 rounded pointer-events-none">
          Showing {nodes.length} / {totalNodes} nodes
        </div>
      )}

      <ReactFlow
        nodes={nodes}
        edges={edges}
        fitView
        fitViewOptions={{ padding: 0.15 }}
        minZoom={0.2}
        maxZoom={1.6}
        nodesDraggable={false}
        nodesConnectable={false}
        onNodeClick={handleNodeClick}
        onNodeMouseEnter={handleNodeMouseEnter}
        onNodeMouseLeave={handleNodeMouseLeave}
        panOnScroll
        onlyRenderVisibleElements
      >
        <Background gap={24} color="#e6dcc8" />
        <MiniMap
          position="bottom-right"
          pannable
          zoomable
          nodeStrokeWidth={2}
          nodeColor={(node) => {
            const score = (node.data as { evaluation?: number }).evaluation;
            return evaluationColor(score);
          }}
          nodeStrokeColor={(node) => {
            if (node.id === selectedNodeId) return "#c9a227";
            return "#8a8f85";
          }}
        />
        <Controls position="top-right" />
      </ReactFlow>

      {hoveredNode && (
        <div
          className="absolute z-50 p-2 rounded-lg border border-line bg-panel shadow-xl pointer-events-none"
          style={{
            left: `${hoveredNode.x + 15}px`,
            top: `${hoveredNode.y - 120}px`,
          }}
        >
          <div className="text-xs font-semibold mb-1 text-center text-ink">{hoveredNode.label}</div>
          <MiniBoard fen={hoveredNode.fen} />
        </div>
      )}
    </div>
  );
}
