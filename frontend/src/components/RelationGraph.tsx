import { useEffect, useMemo, useRef, useState, type KeyboardEvent, type PointerEvent } from "react";
import type { GraphEdge, GraphNode, GraphProjection } from "../api/relations";

const VIEW_WIDTH = 960;
const VIEW_HEIGHT = 720;
const MIN_NODE_DISTANCE = 96;
const NODE_MARGIN = 48;

type Point = { x: number; y: number };

const nodeColors: Record<string, string> = {
  return: "#dc3545",
  customer: "#0077ff",
  agent: "#f59e0b",
  order: "#198754",
  category: "#8b5cf6",
};

const edgeColors: Record<string, string> = {
  MADE_DECISION: "#22c55e",
  PLACED_ORDER: "#0ea5e9",
  HAS_CATEGORY: "#8b5cf6",
  HAS_RETURN_REQUEST: "#ef4444",
  DECIDED_BY: "#f59e0b",
  CUSTOMER_AGENT_PAIR: "#ec4899",
  SAME_CUSTOMER: "#6366f1",
  SAME_AGENT_REASON: "#14b8a6",
  SIMILAR_AGENT_REASON_CATEGORY: "#a855f7",
};

function normalizeEdgeType(value: string) {
  return value.trim().toUpperCase().replace(/[- ]/g, "_");
}

function edgeColor(edge: GraphEdge) {
  return edgeColors[normalizeEdgeType(edge.type)] ?? "#64748b";
}

function edgeKey(edge: GraphEdge, index: number) {
  return `${edge.from}-${edge.to}-${edge.type}-${index}`;
}

function initialPositions(nodes: GraphNode[], returnId: string): Record<string, Point> {
  if (nodes.length === 0) return {};
  const center = { x: VIEW_WIDTH / 2, y: VIEW_HEIGHT / 2 };
  const selected = nodes.find((node) => node.id === `return:${returnId}`) ?? nodes.find((node) => node.type === "return") ?? nodes[0];
  const remaining = nodes
    .filter((node) => node.id !== selected.id)
    .sort((left, right) => Number(left.type === "return") - Number(right.type === "return"));
  const innerCount = Math.min(8, remaining.length);
  const positions: Record<string, Point> = { [selected.id]: center };

  remaining.forEach((node, index) => {
    const inner = index < innerCount;
    const ringIndex = inner ? index : index - innerCount;
    const ringCount = inner ? innerCount : remaining.length - innerCount;
    const radius = inner ? 148 : 270;
    const angle = (ringIndex / Math.max(ringCount, 1)) * Math.PI * 2 - Math.PI / 2;
    positions[node.id] = {
      x: center.x + Math.cos(angle) * radius,
      y: center.y + Math.sin(angle) * radius,
    };
  });

  return positions;
}

function nodeLabel(node: GraphNode) {
  return node.label.length > 18 ? `${node.label.slice(0, 15)}…` : node.label;
}

function clampPoint(point: Point): Point {
  return {
    x: Math.max(NODE_MARGIN, Math.min(VIEW_WIDTH - NODE_MARGIN, point.x)),
    y: Math.max(NODE_MARGIN, Math.min(VIEW_HEIGHT - NODE_MARGIN - 18, point.y)),
  };
}

function placeWithoutOverlap(id: string, desired: Point, positions: Record<string, Point>): Point {
  let candidate = clampPoint(desired);
  for (let iteration = 0; iteration < 5; iteration += 1) {
    let moved = false;
    for (const [otherId, other] of Object.entries(positions)) {
      if (otherId === id) continue;
      const dx = candidate.x - other.x;
      const dy = candidate.y - other.y;
      const distance = Math.hypot(dx, dy);
      if (distance >= MIN_NODE_DISTANCE) continue;
      const angle = distance < 0.01 ? (id.length * 0.73) % (Math.PI * 2) : Math.atan2(dy, dx);
      candidate = clampPoint({
        x: other.x + Math.cos(angle) * MIN_NODE_DISTANCE,
        y: other.y + Math.sin(angle) * MIN_NODE_DISTANCE,
      });
      moved = true;
    }
    if (!moved) break;
  }

  const overlaps = Object.entries(positions).some(([otherId, other]) => (
    otherId !== id && Math.hypot(candidate.x - other.x, candidate.y - other.y) < MIN_NODE_DISTANCE - 1
  ));
  return overlaps ? positions[id] : candidate;
}

export function RelationGraph({ graph }: { graph: GraphProjection }) {
  const svgRef = useRef<SVGSVGElement>(null);
  const dragRef = useRef<{ id: string; pointerId: number } | null>(null);
  const [positions, setPositions] = useState<Record<string, Point>>(() => initialPositions(graph.nodes, graph.returnId));
  const [activeNodeId, setActiveNodeId] = useState(() => graph.nodes.find((node) => node.id === `return:${graph.returnId}`)?.id ?? graph.nodes[0]?.id ?? "");
  const [activeEdgeId, setActiveEdgeId] = useState(() => graph.edges[0] ? edgeKey(graph.edges[0], 0) : "");

  useEffect(() => {
    setPositions(initialPositions(graph.nodes, graph.returnId));
    setActiveNodeId(graph.nodes.find((node) => node.id === `return:${graph.returnId}`)?.id ?? graph.nodes[0]?.id ?? "");
    setActiveEdgeId(graph.edges[0] ? edgeKey(graph.edges[0], 0) : "");
  }, [graph]);

  const activeNode = graph.nodes.find((node) => node.id === activeNodeId);
  const activeEdge = graph.edges.find((edge, index) => edgeKey(edge, index) === activeEdgeId);
  const relationLegend = useMemo(() => {
    const unique = new Map<string, GraphEdge>();
    graph.edges.forEach((edge) => unique.set(normalizeEdgeType(edge.type), edge));
    return [...unique.values()];
  }, [graph.edges]);

  if (graph.nodes.length === 0) {
    return <div className="empty-state"><strong>No relation graph</strong><span>No connected entities were found for this return.</span></div>;
  }

  function pointerPoint(event: PointerEvent<SVGGElement>): Point | null {
    const bounds = svgRef.current?.getBoundingClientRect();
    if (!bounds || bounds.width === 0 || bounds.height === 0) return null;
    return {
      x: ((event.clientX - bounds.left) / bounds.width) * VIEW_WIDTH,
      y: ((event.clientY - bounds.top) / bounds.height) * VIEW_HEIGHT,
    };
  }

  function moveNode(id: string, point: Point) {
    setPositions((current) => ({ ...current, [id]: placeWithoutOverlap(id, point, current) }));
  }

  function handlePointerDown(event: PointerEvent<SVGGElement>, node: GraphNode) {
    event.preventDefault();
    event.currentTarget.setPointerCapture(event.pointerId);
    dragRef.current = { id: node.id, pointerId: event.pointerId };
    setActiveNodeId(node.id);
  }

  function handlePointerMove(event: PointerEvent<SVGGElement>, node: GraphNode) {
    if (dragRef.current?.id !== node.id || dragRef.current.pointerId !== event.pointerId) return;
    const point = pointerPoint(event);
    if (point) moveNode(node.id, point);
  }

  function stopDragging(event: PointerEvent<SVGGElement>) {
    if (dragRef.current?.pointerId === event.pointerId) dragRef.current = null;
  }

  function handleNodeKeyDown(event: KeyboardEvent<SVGGElement>, node: GraphNode) {
    const delta = event.shiftKey ? 30 : 12;
    const movement: Record<string, Point> = {
      ArrowLeft: { x: -delta, y: 0 }, ArrowRight: { x: delta, y: 0 },
      ArrowUp: { x: 0, y: -delta }, ArrowDown: { x: 0, y: delta },
    };
    const change = movement[event.key];
    if (!change) return;
    event.preventDefault();
    const current = positions[node.id];
    moveNode(node.id, { x: current.x + change.x, y: current.y + change.y });
  }

  return (
    <div className="relation-graph-layout">
      <div
        className="relation-graph-canvas"
        role="application"
        aria-label={`Interactive relation graph with ${graph.nodes.length} nodes and ${graph.edges.length} edges`}
      >
        <div className="graph-instructions">Drag nodes to rearrange · select an edge for details</div>
        <svg ref={svgRef} viewBox={`0 0 ${VIEW_WIDTH} ${VIEW_HEIGHT}`}>
          <defs>
            <marker id="graph-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
              <path d="M 0 0 L 10 5 L 0 10 z" />
            </marker>
          </defs>

          <g className="graph-edges">
            {graph.edges.map((edge, index) => {
              const from = positions[edge.from];
              const to = positions[edge.to];
              if (!from || !to) return null;
              const key = edgeKey(edge, index);
              const active = key === activeEdgeId;
              const color = edgeColor(edge);
              return (
                <g
                  className={active ? "graph-edge active" : "graph-edge"}
                  key={key}
                  role="button"
                  tabIndex={0}
                  aria-label={`${edge.label || edge.type}: ${edge.reason}`}
                  onClick={() => setActiveEdgeId(key)}
                  onFocus={() => setActiveEdgeId(key)}
                  onPointerEnter={() => setActiveEdgeId(key)}
                >
                  <title>{`${edge.label || edge.type}: ${edge.reason}`}</title>
                  <line className="graph-edge-hitbox" x1={from.x} y1={from.y} x2={to.x} y2={to.y} />
                  <line style={{ stroke: color }} x1={from.x} y1={from.y} x2={to.x} y2={to.y} markerEnd="url(#graph-arrow)" />
                </g>
              );
            })}
          </g>

          <g className="graph-nodes">
            {graph.nodes.map((node) => {
              const point = positions[node.id];
              const selected = node.id === activeNodeId;
              return (
                <g
                  className={selected ? "graph-node active" : "graph-node"}
                  key={node.id}
                  transform={`translate(${point.x} ${point.y})`}
                  role="button"
                  tabIndex={0}
                  aria-label={`${node.type}: ${node.label}. ${node.summary}. Drag or use arrow keys to move.`}
                  onClick={() => setActiveNodeId(node.id)}
                  onKeyDown={(event) => handleNodeKeyDown(event, node)}
                  onPointerDown={(event) => handlePointerDown(event, node)}
                  onPointerMove={(event) => handlePointerMove(event, node)}
                  onPointerUp={stopDragging}
                  onPointerCancel={stopDragging}
                >
                  <title>{`${node.label}: ${node.summary}`}</title>
                  <circle r={node.type === "return" ? 31 : 27} fill={nodeColors[node.type] ?? "#6c757d"} />
                  <text className="graph-node-type" y="4" textAnchor="middle">{node.type.slice(0, 3).toUpperCase()}</text>
                  <text className="graph-node-label" y="47" textAnchor="middle">{nodeLabel(node)}</text>
                </g>
              );
            })}
          </g>
        </svg>

        <div className="graph-selection" aria-label="Relation details">
          {activeEdge ? (
            <>
              <i style={{ background: edgeColor(activeEdge) }} />
              <div><strong>{activeEdge.label || activeEdge.type}</strong><span>{activeEdge.reason}</span></div>
            </>
          ) : activeNode ? (
            <div><strong>{activeNode.label}</strong><span>{activeNode.summary}</span></div>
          ) : null}
        </div>
      </div>

      <div className="graph-meta">
        <div className="graph-legend" aria-label="Graph legend">
          {Object.entries(nodeColors).filter(([type]) => graph.nodes.some((node) => node.type === type)).map(([type, color]) => (
            <span key={type}><i style={{ background: color }} />{type}</span>
          ))}
        </div>
        <div className="graph-relation-legend" aria-label="Relation type legend">
          {relationLegend.map((edge) => (
            <button key={normalizeEdgeType(edge.type)} type="button" onClick={() => {
              const index = graph.edges.findIndex((candidate) => candidate === edge);
              setActiveEdgeId(edgeKey(edge, index));
            }}>
              <i style={{ background: edgeColor(edge) }} />{edge.label || edge.type}
            </button>
          ))}
        </div>
        {graph.truncated && <div className="api-notice warning" role="status">Graph is truncated to {graph.limit} nodes.</div>}
      </div>
    </div>
  );
}
