import { type DragEvent } from 'react'
import { NODE_TYPES, NODE_COLORS } from './nodeTypes'

/** Sidebar palette of draggable node types for the flow editor. */
export default function NodePalette() {
  function onDragStart(event: DragEvent, nodeType: string) {
    event.dataTransfer.setData('application/reactflow-type', nodeType)
    event.dataTransfer.effectAllowed = 'move'
  }

  return (
    <div className="w-52 bg-white border-r border-gray-200 flex flex-col shrink-0">
      <div className="px-3 py-2 border-b border-gray-200">
        <h3 className="text-xs font-semibold text-gray-500 uppercase tracking-wider">Nodes</h3>
      </div>
      <div className="flex-1 overflow-y-auto p-2 space-y-1">
        {NODE_TYPES.map((info) => {
          const colors = NODE_COLORS[info.color] ?? NODE_COLORS.blue
          return (
            <div
              key={info.type}
              draggable
              onDragStart={(e) => onDragStart(e, info.type)}
              className={`flex items-center gap-2 px-2.5 py-2 rounded-md border ${colors.border} ${colors.bg} cursor-grab active:cursor-grabbing hover:shadow-sm transition-shadow select-none`}
              title={info.description}
            >
              <svg className={`w-4 h-4 shrink-0 ${colors.text}`} viewBox="0 0 20 20" fill="currentColor">
                <path fillRule="evenodd" d={info.iconPath} clipRule="evenodd" />
              </svg>
              <span className="text-sm font-medium text-gray-700">{info.label}</span>
            </div>
          )
        })}
      </div>
    </div>
  )
}
