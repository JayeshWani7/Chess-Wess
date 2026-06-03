/**
 * Timeline memory management utilities.
 *
 * Strategy:
 * - Keep a maximum of MAX_HOT_TIMELINES timelines with their full node lists
 *   in memory ("hot"). These are the most recently accessed timelines.
 * - All other timelines are "cold": only their summary (id, name, node_count,
 *   first+last node for branch edges) is retained. Full nodes are evicted.
 * - The active timeline and the selected-node timeline are always hot.
 * - Access order is tracked via an LRU list (most-recent at front).
 */

import type { TimelineData, TimelineNode } from "./gameStore";

/** Maximum number of timelines to keep fully loaded in memory. */
export const MAX_HOT_TIMELINES = 8;

/** Maximum nodes rendered to React Flow at once. */
export const MAX_RENDER_NODES = 300;

/** Minimum nodes always rendered per timeline (active path + selected). */
export const MIN_NODES_PER_TIMELINE = 5;

// ── LRU tracker ──────────────────────────────────────────────────────────────

export class LRUTracker {
  private order: string[] = []; // most-recent first
  private max: number;

  constructor(max: number) {
    this.max = max;
  }

  /** Record an access. Returns the timeline ID that was evicted (if any). */
  touch(id: string): string | null {
    this.order = this.order.filter((x) => x !== id);
    this.order.unshift(id);
    if (this.order.length > this.max) {
      return this.order.pop() ?? null;
    }
    return null;
  }

  /** Mark a timeline as pinned (won't be evicted). */
  pin(id: string) {
    this.touch(id); // push to front; it won't be at the back
  }

  isHot(id: string): boolean {
    return this.order.includes(id);
  }

  hotSet(): Set<string> {
    return new Set(this.order);
  }

  remove(id: string) {
    this.order = this.order.filter((x) => x !== id);
  }

  reset() {
    this.order = [];
  }
}

// ── Node eviction ─────────────────────────────────────────────────────────────

/**
 * Given a full TimelineData[], apply eviction so only hot timelines keep their
 * node arrays. Cold timelines retain only a 2-node stub (root + latest) so
 * the graph can still draw summary edges and branch points.
 */
export function applyEviction(
  timelines: TimelineData[],
  hotSet: Set<string>,
  pinnedIds: Set<string>
): TimelineData[] {
  return timelines.map((t) => {
    if (hotSet.has(t.timeline_id) || pinnedIds.has(t.timeline_id)) {
      return t; // keep full nodes
    }
    // Cold: keep only stub nodes
    const stub = stubNodes(t.nodes);
    return {
      ...t,
      nodes: stub,
      nodes_partial: true,
    };
  });
}

/** Returns up to 2 representative nodes for a cold timeline (root + tail). */
function stubNodes(nodes: TimelineNode[]): TimelineNode[] {
  if (nodes.length === 0) return [];
  if (nodes.length <= 2) return nodes;
  return [nodes[0], nodes[nodes.length - 1]];
}

// ── Render budget ─────────────────────────────────────────────────────────────

export interface RenderBudgetOptions {
  timelines: TimelineData[];
  activeTimelineId: string | null;
  selectedNodeId: string | null;
  maxNodes: number;
}

/**
 * Applies a render budget: trims timeline node lists so the total rendered
 * node count stays within maxNodes. Priority:
 *  1. Active timeline — all nodes always included
 *  2. Timeline containing the selected node — all nodes included
 *  3. Other timelines — trimmed proportionally from the oldest nodes first,
 *     always keeping at least MIN_NODES_PER_TIMELINE (latest moves)
 *
 * Returns a new array (original data is not mutated).
 */
export function applyRenderBudget({
  timelines,
  activeTimelineId,
  selectedNodeId,
  maxNodes,
}: RenderBudgetOptions): TimelineData[] {
  const totalNodes = timelines.reduce((s, t) => s + t.nodes.length, 0);
  if (totalNodes <= maxNodes) return timelines; // already within budget

  // Find the timeline that owns the selected node
  const selectedTimelineId = selectedNodeId
    ? timelines.find((t) => t.nodes.some((n) => n.id === selectedNodeId))
        ?.timeline_id ?? null
    : null;

  // Pinned timelines always get their full quota
  const pinned = new Set<string>(
    [activeTimelineId, selectedTimelineId].filter(Boolean) as string[]
  );

  const pinnedNodes = timelines
    .filter((t) => pinned.has(t.timeline_id))
    .reduce((s, t) => s + t.nodes.length, 0);

  const remaining = Math.max(0, maxNodes - pinnedNodes);
  const otherTimelines = timelines.filter((t) => !pinned.has(t.timeline_id));
  const nodesPerOther =
    otherTimelines.length > 0
      ? Math.max(
          MIN_NODES_PER_TIMELINE,
          Math.floor(remaining / otherTimelines.length)
        )
      : 0;

  return timelines.map((t) => {
    if (pinned.has(t.timeline_id)) return t;
    if (t.nodes.length <= nodesPerOther) return t;
    // Keep the latest N nodes (most relevant for display)
    return {
      ...t,
      nodes: t.nodes.slice(-nodesPerOther),
      nodes_partial: true,
    };
  });
}

// ── Active-path extraction ────────────────────────────────────────────────────

/**
 * Walks parent_node_id pointers to reconstruct the path from root to nodeId.
 * Returns an ordered array of node IDs from root → nodeId.
 */
export function getActivePath(
  nodeId: string,
  nodesById: Record<string, TimelineNode>
): string[] {
  const path: string[] = [];
  let current: TimelineNode | undefined = nodesById[nodeId];
  while (current) {
    path.unshift(current.id);
    current = current.parent_node_id
      ? nodesById[current.parent_node_id]
      : undefined;
  }
  return path;
}

// ── Summary-only representation ───────────────────────────────────────────────

export interface TimelineSummary {
  timeline_id: string;
  timeline_name: string | null | undefined;
  node_count: number;
  latest_fen: string | null;
  latest_san: string | null;
  is_partial: boolean;
}

/** Derive lightweight summaries from the full timeline list. */
export function buildSummaries(timelines: TimelineData[]): TimelineSummary[] {
  return timelines.map((t) => {
    const last = t.nodes.length > 0 ? t.nodes[t.nodes.length - 1] : null;
    return {
      timeline_id: t.timeline_id,
      timeline_name: t.timeline_name,
      node_count: t.node_count ?? t.nodes.length,
      latest_fen: last?.board_state ?? null,
      latest_san: last?.move?.san ?? null,
      is_partial: t.nodes_partial ?? false,
    };
  });
}
