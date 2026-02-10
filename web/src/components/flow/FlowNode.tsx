import { memo } from 'react'
import { Handle, Position, type NodeProps, type Node } from '@xyflow/react'
import { getNodeTypeInfo, NODE_COLORS } from './nodeTypes'

/** Data shape stored in every flow node. */
export interface FlowNodeData extends Record<string, unknown> {
  label: string
  entity_id?: number | null
  entity_type?: string
  config?: Record<string, unknown>
  /** Dynamic output handles (for time_switch, ivr_menu). */
  outputHandles?: { id: string; label: string }[]
  /** Validation issues attached to this node. */
  validationIssues?: { severity: 'error' | 'warning'; message: string }[]
}

export type FlowNodeType = Node<FlowNodeData>

/** Renders a single flow node with typed input/output handles. */
function FlowNodeComponent({ data, type, selected }: NodeProps<FlowNodeType>) {
  const info = getNodeTypeInfo(type ?? '')
  if (!info) return null

  const colors = NODE_COLORS[info.color] ?? NODE_COLORS.blue
  const hasValidationError = data.validationIssues?.some((i) => i.severity === 'error')
  const hasValidationWarning = data.validationIssues?.some((i) => i.severity === 'warning')

  // Determine output handles: use dynamic ones from data, or fall back to static from info.
  const outputHandles = data.outputHandles ?? info.outputHandles ?? []
  const isTerminal = info.outputs === 0
  const isEntry = info.type === 'inbound_number'

  return (
    <div
      className={`rounded-lg border-2 shadow-sm min-w-[180px] max-w-[220px] ${colors.bg} ${
        hasValidationError
          ? 'border-red-500'
          : hasValidationWarning
            ? 'border-amber-500'
            : selected
              ? 'border-blue-500'
              : colors.border
      } transition-colors`}
    >
      {/* Input handle — all nodes except entry nodes */}
      {!isEntry && (
        <Handle
          type="target"
          position={Position.Left}
          id="incoming"
          className="!w-3 !h-3 !bg-gray-400 !border-2 !border-white"
        />
      )}

      {/* Node header */}
      <div className={`flex items-center gap-2 px-3 py-2 ${colors.text}`}>
        <svg className="w-4 h-4 shrink-0" viewBox="0 0 20 20" fill="currentColor">
          <path fillRule="evenodd" d={info.iconPath} clipRule="evenodd" />
        </svg>
        <div className="min-w-0 flex-1">
          <div className="text-[11px] font-semibold uppercase tracking-wider opacity-60">
            {info.label}
          </div>
          <div className="text-sm font-medium truncate text-gray-900">{data.label || 'Untitled'}</div>
        </div>
      </div>

      {/* Output handles */}
      {!isTerminal && outputHandles.length > 0 && (
        <div className="border-t border-gray-200/60 px-3 py-1.5 space-y-1">
          {outputHandles.map((h) => (
            <div key={h.id} className="relative flex items-center justify-end pr-1">
              <span className="text-[10px] text-gray-500 mr-1">{h.label}</span>
              <Handle
                type="source"
                position={Position.Right}
                id={h.id}
                className={`!w-2.5 !h-2.5 ${colors.handle} !border-2 !border-white`}
                style={{ top: 'auto', position: 'relative', transform: 'none' }}
              />
            </div>
          ))}
        </div>
      )}

      {/* Single output for simple passthrough nodes (when no named handles) */}
      {!isTerminal && outputHandles.length === 0 && info.outputs !== 0 && (
        <Handle
          type="source"
          position={Position.Right}
          id="next"
          className={`!w-3 !h-3 ${colors.handle} !border-2 !border-white`}
        />
      )}

      {/* Terminal indicator */}
      {isTerminal && (
        <div className="border-t border-gray-200/60 px-3 py-1 text-center">
          <span className="text-[10px] text-gray-400 uppercase tracking-wider">End</span>
        </div>
      )}

      {/* Validation badge */}
      {hasValidationError && (
        <div className="absolute -top-2 -right-2 w-5 h-5 rounded-full bg-red-500 flex items-center justify-center">
          <span className="text-white text-[10px] font-bold">!</span>
        </div>
      )}
      {!hasValidationError && hasValidationWarning && (
        <div className="absolute -top-2 -right-2 w-5 h-5 rounded-full bg-amber-500 flex items-center justify-center">
          <span className="text-white text-[10px] font-bold">!</span>
        </div>
      )}
    </div>
  )
}

export default memo(FlowNodeComponent)
