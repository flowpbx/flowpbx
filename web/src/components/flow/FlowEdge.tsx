import { memo } from 'react'
import {
  BaseEdge,
  EdgeLabelRenderer,
  getSmoothStepPath,
  type EdgeProps,
  type Edge,
} from '@xyflow/react'

export type FlowEdgeData = {
  label?: string
}

export type FlowEdgeType = Edge<FlowEdgeData>

/** Custom edge with a label badge rendered at the midpoint. */
function FlowEdgeComponent({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  data,
  selected,
}: EdgeProps<FlowEdgeType>) {
  const [edgePath, labelX, labelY] = getSmoothStepPath({
    sourceX,
    sourceY,
    targetX,
    targetY,
    sourcePosition,
    targetPosition,
    borderRadius: 8,
  })

  const label = data?.label

  return (
    <>
      <BaseEdge
        id={id}
        path={edgePath}
        style={{
          stroke: selected ? '#3b82f6' : '#94a3b8',
          strokeWidth: selected ? 2 : 1.5,
        }}
      />
      {label && (
        <EdgeLabelRenderer>
          <div
            className="absolute bg-white border border-gray-200 rounded px-1.5 py-0.5 text-[10px] text-gray-600 font-medium shadow-sm pointer-events-none"
            style={{
              transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
            }}
          >
            {label}
          </div>
        </EdgeLabelRenderer>
      )}
    </>
  )
}

export default memo(FlowEdgeComponent)
