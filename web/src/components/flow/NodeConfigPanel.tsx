import { useState, useEffect } from 'react'
import type { Node } from '@xyflow/react'
import type { FlowNodeData } from './FlowNode'
import { getNodeTypeInfo, NODE_COLORS } from './nodeTypes'
import EntitySelector from './EntitySelector'
import { TextInput } from '../FormFields'

interface Props {
  node: Node
  onUpdate: (nodeId: string, data: Partial<FlowNodeData>) => void
  onClose: () => void
}

/** Side panel for configuring a selected flow node. */
export default function NodeConfigPanel({ node, onUpdate, onClose }: Props) {
  const info = getNodeTypeInfo(node.type ?? '')
  const colors = info ? NODE_COLORS[info.color] ?? NODE_COLORS.blue : NODE_COLORS.blue
  const data = node.data as FlowNodeData

  const [label, setLabel] = useState(data.label ?? '')

  useEffect(() => {
    setLabel((node.data as FlowNodeData).label ?? '')
  }, [node.id, node.data])

  function handleLabelChange(value: string) {
    setLabel(value)
    onUpdate(node.id, { label: value })
  }

  function handleEntitySelect(entityId: number | null) {
    onUpdate(node.id, { entity_id: entityId })
  }

  function handleConfigChange(key: string, value: unknown) {
    onUpdate(node.id, {
      config: { ...data.config, [key]: value },
    })
  }

  // Dynamic handle management for time_switch and ivr_menu
  function addOutputHandle(id: string, handleLabel: string) {
    const existing = data.outputHandles ?? []
    if (existing.some((h) => h.id === id)) return
    onUpdate(node.id, {
      outputHandles: [...existing, { id, label: handleLabel }],
    })
  }

  function removeOutputHandle(id: string) {
    const existing = data.outputHandles ?? []
    onUpdate(node.id, {
      outputHandles: existing.filter((h) => h.id !== id),
    })
  }

  return (
    <div className="w-80 bg-white border-l border-gray-200 flex flex-col shrink-0">
      {/* Header */}
      <div className={`flex items-center justify-between px-4 py-3 border-b border-gray-200 ${colors.bg}`}>
        <div className="flex items-center gap-2">
          {info && (
            <svg className={`w-4 h-4 ${colors.text}`} viewBox="0 0 20 20" fill="currentColor">
              <path fillRule="evenodd" d={info.iconPath} clipRule="evenodd" />
            </svg>
          )}
          <h3 className="text-sm font-semibold text-gray-900">{info?.label ?? 'Node'}</h3>
        </div>
        <button
          type="button"
          onClick={onClose}
          className="text-gray-400 hover:text-gray-600 transition-colors"
        >
          <svg className="w-5 h-5" viewBox="0 0 20 20" fill="currentColor">
            <path fillRule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clipRule="evenodd" />
          </svg>
        </button>
      </div>

      {/* Body */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {/* Label */}
        <TextInput
          label="Label"
          id="node-label"
          value={label}
          onChange={(e) => handleLabelChange(e.currentTarget.value)}
          placeholder="Enter label..."
        />

        {/* Entity selector for entity-linked nodes */}
        {info?.entityType && (
          <EntitySelector
            entityType={info.entityType}
            selectedId={(data.entity_id as number) ?? null}
            onSelect={handleEntitySelect}
          />
        )}

        {/* Node-type specific config */}
        {node.type === 'transfer' && (
          <TextInput
            label="Transfer To"
            id="transfer-to"
            value={(data.config?.transfer_to as string) ?? ''}
            onChange={(e) => handleConfigChange('transfer_to', e.currentTarget.value)}
            placeholder="Number or extension"
          />
        )}

        {node.type === 'hangup' && (
          <TextInput
            label="Cause Code"
            id="hangup-cause"
            value={(data.config?.cause_code as string) ?? ''}
            onChange={(e) => handleConfigChange('cause_code', e.currentTarget.value)}
            placeholder="Normal (default)"
          />
        )}

        {node.type === 'set_caller_id' && (
          <>
            <TextInput
              label="Caller ID Name"
              id="cid-name"
              value={(data.config?.caller_id_name as string) ?? ''}
              onChange={(e) => handleConfigChange('caller_id_name', e.currentTarget.value)}
              placeholder="Display name"
            />
            <TextInput
              label="Caller ID Number"
              id="cid-number"
              value={(data.config?.caller_id_num as string) ?? ''}
              onChange={(e) => handleConfigChange('caller_id_num', e.currentTarget.value)}
              placeholder="Number"
            />
          </>
        )}

        {node.type === 'play_message' && (
          <TextInput
            label="Audio Prompt"
            id="prompt-file"
            value={(data.config?.prompt as string) ?? ''}
            onChange={(e) => handleConfigChange('prompt', e.currentTarget.value)}
            placeholder="Prompt filename"
          />
        )}

        {/* Dynamic outputs for time_switch */}
        {node.type === 'time_switch' && (
          <DynamicOutputEditor
            handles={data.outputHandles ?? [{ id: 'default', label: 'Default' }]}
            onAdd={addOutputHandle}
            onRemove={removeOutputHandle}
            addLabel="Add Time Rule"
            idPrefix="rule_"
          />
        )}

        {/* Dynamic outputs for ivr_menu */}
        {node.type === 'ivr_menu' && (
          <DynamicOutputEditor
            handles={data.outputHandles ?? [
              { id: 'timeout', label: 'Timeout' },
              { id: 'invalid', label: 'Invalid' },
            ]}
            onAdd={addOutputHandle}
            onRemove={removeOutputHandle}
            addLabel="Add Digit"
            idPrefix="digit_"
            protectedIds={['timeout', 'invalid']}
          />
        )}

        {/* Validation issues */}
        {data.validationIssues && data.validationIssues.length > 0 && (
          <div className="space-y-1">
            <p className="text-xs font-medium text-gray-500 uppercase tracking-wider">Issues</p>
            {data.validationIssues.map((issue, i) => (
              <div
                key={i}
                className={`text-xs px-2 py-1 rounded ${
                  issue.severity === 'error'
                    ? 'bg-red-50 text-red-700'
                    : 'bg-amber-50 text-amber-700'
                }`}
              >
                {issue.message}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

/** Editor for dynamic output handles (time rules, IVR digits). */
function DynamicOutputEditor({
  handles,
  onAdd,
  onRemove,
  addLabel,
  idPrefix,
  protectedIds = [],
}: {
  handles: { id: string; label: string }[]
  onAdd: (id: string, label: string) => void
  onRemove: (id: string) => void
  addLabel: string
  idPrefix: string
  protectedIds?: string[]
}) {
  const [newLabel, setNewLabel] = useState('')

  function handleAdd() {
    if (!newLabel.trim()) return
    const id = idPrefix + newLabel.trim().toLowerCase().replace(/[^a-z0-9]/g, '_')
    onAdd(id, newLabel.trim())
    setNewLabel('')
  }

  return (
    <div className="space-y-2">
      <p className="text-xs font-medium text-gray-500 uppercase tracking-wider">Outputs</p>
      <div className="space-y-1">
        {handles.map((h) => (
          <div key={h.id} className="flex items-center justify-between text-sm">
            <span className="text-gray-700">{h.label}</span>
            {!protectedIds.includes(h.id) && (
              <button
                type="button"
                onClick={() => onRemove(h.id)}
                className="text-xs text-red-500 hover:text-red-700"
              >
                Remove
              </button>
            )}
          </div>
        ))}
      </div>
      <div className="flex gap-2">
        <input
          type="text"
          value={newLabel}
          onChange={(e) => setNewLabel(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleAdd()}
          placeholder="Label..."
          className="flex-1 text-sm rounded border border-gray-300 px-2 py-1 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
        />
        <button
          type="button"
          onClick={handleAdd}
          className="text-xs text-blue-600 hover:text-blue-800 font-medium whitespace-nowrap"
        >
          {addLabel}
        </button>
      </div>
    </div>
  )
}
