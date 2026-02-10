import { useState, useCallback, useRef, type DragEvent } from 'react'
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  addEdge,
  useNodesState,
  useEdgesState,
  type Connection,
  type Node,
  type Edge,
  type NodeTypes,
  type EdgeTypes,
  type OnSelectionChangeParams,
  type ReactFlowInstance,
  BackgroundVariant,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'

import FlowNodeComponent, { type FlowNodeData } from './FlowNode'
import FlowEdgeComponent from './FlowEdge'
import NodePalette from './NodePalette'
import NodeConfigPanel from './NodeConfigPanel'
import { getNodeTypeInfo } from './nodeTypes'

/** React Flow node type registry — all flow nodes use the same component. */
const nodeTypes: NodeTypes = {
  inbound_number: FlowNodeComponent,
  time_switch: FlowNodeComponent,
  ivr_menu: FlowNodeComponent,
  ring_group: FlowNodeComponent,
  extension: FlowNodeComponent,
  voicemail: FlowNodeComponent,
  play_message: FlowNodeComponent,
  conference: FlowNodeComponent,
  transfer: FlowNodeComponent,
  hangup: FlowNodeComponent,
  set_caller_id: FlowNodeComponent,
}

const edgeTypes: EdgeTypes = {
  flow: FlowEdgeComponent,
}

/** Default edge options — smooth step with label support. */
const defaultEdgeOptions = {
  type: 'flow',
  animated: false,
}

interface Props {
  /** Initial nodes to render. */
  initialNodes: Node[]
  /** Initial edges to render. */
  initialEdges: Edge[]
  /** Called when the flow graph changes (nodes or edges updated). */
  onChange?: (nodes: Node[], edges: Edge[]) => void
  /** Validation issues indexed by node ID. */
  validationIssues?: Record<string, { severity: 'error' | 'warning'; message: string }[]>
}

let nodeIdCounter = 0

function generateNodeId(): string {
  return `node_${Date.now()}_${++nodeIdCounter}`
}

/** Main flow editor canvas with drag-and-drop, node palette, and config panel. */
export default function FlowCanvas({ initialNodes, initialEdges, onChange, validationIssues }: Props) {
  const reactFlowWrapper = useRef<HTMLDivElement>(null)
  const [reactFlowInstance, setReactFlowInstance] = useState<ReactFlowInstance | null>(null)
  const [nodes, setNodes, onNodesChange] = useNodesState(initialNodes)
  const [edges, setEdges, onEdgesChange] = useEdgesState(initialEdges)
  const [selectedNode, setSelectedNode] = useState<Node | null>(null)

  // Notify parent of changes
  const notifyChange = useCallback(
    (n: Node[], e: Edge[]) => {
      onChange?.(n, e)
    },
    [onChange],
  )

  const onConnect = useCallback(
    (connection: Connection) => {
      // Determine edge label from source handle name
      const sourceNode = nodes.find((n) => n.id === connection.source)
      let label = ''
      if (sourceNode) {
        const data = sourceNode.data as FlowNodeData
        const handle = data.outputHandles?.find((h) => h.id === connection.sourceHandle)
        if (handle && handle.id !== 'next') {
          label = handle.label
        }
      }

      const newEdge = {
        ...connection,
        type: 'flow',
        data: { label },
      }
      setEdges((eds) => {
        const updated = addEdge(newEdge, eds)
        notifyChange(nodes, updated)
        return updated
      })
    },
    [nodes, setEdges, notifyChange],
  )

  const handleNodesChange: typeof onNodesChange = useCallback(
    (changes) => {
      onNodesChange(changes)
      queueMicrotask(() => {
        setNodes((current) => {
          notifyChange(current, edges)
          return current
        })
      })
    },
    [onNodesChange, setNodes, edges, notifyChange],
  )

  const handleEdgesChange: typeof onEdgesChange = useCallback(
    (changes) => {
      onEdgesChange(changes)
      queueMicrotask(() => {
        setEdges((current) => {
          notifyChange(nodes, current)
          return current
        })
      })
    },
    [onEdgesChange, setEdges, nodes, notifyChange],
  )

  // Selection change — open config panel for selected node
  const onSelectionChange = useCallback(
    ({ nodes: selected }: OnSelectionChangeParams) => {
      if (selected.length === 1) {
        setSelectedNode(selected[0])
      } else {
        setSelectedNode(null)
      }
    },
    [],
  )

  // Update the selected node in both state and the canvas
  const onUpdateNode = useCallback(
    (nodeId: string, data: Partial<FlowNodeData>) => {
      setNodes((nds) => {
        const updated = nds.map((n) => {
          if (n.id === nodeId) {
            const existingData = n.data as FlowNodeData
            const newData: FlowNodeData = { ...existingData, ...data }
            // Merge config objects rather than replace
            if (data.config && existingData.config) {
              newData.config = { ...existingData.config, ...data.config }
            }
            const updatedNode = { ...n, data: newData }
            setSelectedNode(updatedNode)
            return updatedNode
          }
          return n
        })
        notifyChange(updated, edges)
        return updated
      })
    },
    [setNodes, edges, notifyChange],
  )

  // Drag-and-drop from palette
  const onDragOver = useCallback((event: DragEvent) => {
    event.preventDefault()
    event.dataTransfer.dropEffect = 'move'
  }, [])

  const onDrop = useCallback(
    (event: DragEvent) => {
      event.preventDefault()

      const type = event.dataTransfer.getData('application/reactflow-type')
      if (!type || !reactFlowInstance || !reactFlowWrapper.current) return

      const info = getNodeTypeInfo(type)
      if (!info) return

      const bounds = reactFlowWrapper.current.getBoundingClientRect()
      const position = reactFlowInstance.screenToFlowPosition({
        x: event.clientX - bounds.left,
        y: event.clientY - bounds.top,
      })

      // Build initial output handles for dynamic nodes
      let outputHandles: { id: string; label: string }[] | undefined
      if (type === 'time_switch') {
        outputHandles = [{ id: 'default', label: 'Default' }]
      } else if (type === 'ivr_menu') {
        outputHandles = [
          { id: 'timeout', label: 'Timeout' },
          { id: 'invalid', label: 'Invalid' },
        ]
      }

      const nodeData: FlowNodeData = {
        label: info.label,
        entity_type: info.entityType,
        outputHandles,
      }

      const newNode: Node = {
        id: generateNodeId(),
        type,
        position,
        data: nodeData,
      }

      setNodes((nds) => {
        const updated = [...nds, newNode]
        notifyChange(updated, edges)
        return updated
      })

      // Auto-select the new node to open config panel
      setSelectedNode(newNode)
    },
    [reactFlowInstance, setNodes, edges, notifyChange],
  )

  // Apply validation issues to nodes for rendering
  const nodesWithValidation = validationIssues
    ? nodes.map((n) => ({
        ...n,
        data: {
          ...n.data,
          validationIssues: validationIssues[n.id],
        },
      }))
    : nodes

  return (
    <div className="flex flex-1 h-full">
      <NodePalette />

      <div ref={reactFlowWrapper} className="flex-1 h-full">
        <ReactFlow
          nodes={nodesWithValidation}
          edges={edges}
          onNodesChange={handleNodesChange}
          onEdgesChange={handleEdgesChange}
          onConnect={onConnect}
          onInit={setReactFlowInstance}
          onDrop={onDrop}
          onDragOver={onDragOver}
          onSelectionChange={onSelectionChange}
          nodeTypes={nodeTypes}
          edgeTypes={edgeTypes}
          defaultEdgeOptions={defaultEdgeOptions}
          fitView
          deleteKeyCode="Delete"
          multiSelectionKeyCode="Shift"
          snapToGrid
          snapGrid={[16, 16]}
        >
          <Background variant={BackgroundVariant.Dots} gap={16} size={1} color="#e5e7eb" />
          <Controls />
          <MiniMap
            nodeStrokeWidth={3}
            zoomable
            pannable
            className="!bg-gray-50 !border-gray-200"
          />
        </ReactFlow>
      </div>

      {selectedNode && (
        <NodeConfigPanel
          node={selectedNode}
          onUpdate={onUpdateNode}
          onClose={() => setSelectedNode(null)}
        />
      )}
    </div>
  )
}
