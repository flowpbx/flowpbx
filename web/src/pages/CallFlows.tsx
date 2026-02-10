import { useState, useEffect, useCallback, useRef } from 'react'
import type { Node, Edge } from '@xyflow/react'
import { FlowCanvas } from '../components/flow'
import type { FlowNodeData } from '../components/flow'
import {
  listFlows,
  getFlow,
  createFlow,
  updateFlow,
  deleteFlow,
  publishFlow,
  validateFlow,
  ApiError,
} from '../api'
import type { CallFlow, FlowValidationIssue } from '../api'

/** Serialize nodes and edges to the flow_data JSON format matching the Go backend. */
function serializeFlowData(nodes: Node[], edges: Edge[]): string {
  return JSON.stringify({
    nodes: nodes.map((n) => {
      const d = n.data as FlowNodeData
      return {
        id: n.id,
        type: n.type,
        position: n.position,
        data: {
          label: d.label,
          entity_id: d.entity_id ?? undefined,
          entity_type: d.entity_type ?? undefined,
          config: d.config ?? undefined,
          outputHandles: d.outputHandles ?? undefined,
        },
      }
    }),
    edges: edges.map((e) => ({
      id: e.id,
      source: e.source,
      target: e.target,
      sourceHandle: e.sourceHandle,
      targetHandle: e.targetHandle,
      label: (e.data as { label?: string })?.label ?? undefined,
    })),
  })
}

/** Parse flow_data JSON from the backend into React Flow nodes and edges. */
function parseFlowData(flowData: string): { nodes: Node[]; edges: Edge[] } {
  try {
    const parsed = JSON.parse(flowData)
    const nodes: Node[] = (parsed.nodes ?? []).map((n: Record<string, unknown>) => {
      const nd = n.data as Record<string, unknown> | undefined
      const nodeData: FlowNodeData = {
        label: (nd?.label as string) ?? '',
        entity_id: nd?.entity_id as number | undefined,
        entity_type: nd?.entity_type as string | undefined,
        config: nd?.config as Record<string, unknown> | undefined,
        outputHandles: nd?.outputHandles as { id: string; label: string }[] | undefined,
      }
      return {
        id: n.id as string,
        type: n.type as string,
        position: n.position as { x: number; y: number },
        data: nodeData,
      }
    })
    const edges: Edge[] = (parsed.edges ?? []).map((e: Record<string, unknown>) => ({
      id: e.id as string,
      source: e.source as string,
      target: e.target as string,
      sourceHandle: e.sourceHandle as string,
      targetHandle: e.targetHandle as string,
      type: 'flow',
      data: { label: (e.label as string) ?? '' },
    }))
    return { nodes, edges }
  } catch {
    return { nodes: [], edges: [] }
  }
}

export default function CallFlows() {
  // View state: list or editor
  const [view, setView] = useState<'list' | 'editor'>('list')

  // Flow list state
  const [flows, setFlows] = useState<CallFlow[]>([])
  const [loading, setLoading] = useState(true)

  // Editor state
  const [activeFlow, setActiveFlow] = useState<CallFlow | null>(null)
  const [flowName, setFlowName] = useState('')
  const [saving, setSaving] = useState(false)
  const [publishing, setPublishing] = useState(false)
  const [validating, setValidating] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [validationIssues, setValidationIssues] = useState<Record<string, { severity: 'error' | 'warning'; message: string }[]>>({})
  const [validationResult, setValidationResult] = useState<{ valid: boolean; issues: FlowValidationIssue[] } | null>(null)

  // Track current nodes/edges for saving
  const nodesRef = useRef<Node[]>([])
  const edgesRef = useRef<Edge[]>([])

  // Auto-save timer
  const autoSaveTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Load flow list
  function loadFlows() {
    setLoading(true)
    listFlows()
      .then((data) => setFlows(data))
      .catch(() => setFlows([]))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    loadFlows()
  }, [])

  // Open a flow in the editor
  async function openFlow(flow: CallFlow) {
    try {
      const full = await getFlow(flow.id)
      setActiveFlow(full)
      setFlowName(full.name)

      const { nodes, edges } = parseFlowData(full.flow_data)
      nodesRef.current = nodes
      edgesRef.current = edges

      setValidationIssues({})
      setValidationResult(null)
      setError('')
      setSuccess('')
      setView('editor')
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'failed to load flow')
    }
  }

  // Create a new flow
  async function handleCreate() {
    setError('')
    try {
      const flow = await createFlow({ name: 'New Flow' })
      await openFlow(flow)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'failed to create flow')
    }
  }

  // Delete a flow
  async function handleDelete(flow: CallFlow) {
    if (!confirm(`Delete flow "${flow.name}"? This cannot be undone.`)) return
    try {
      await deleteFlow(flow.id)
      loadFlows()
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'failed to delete flow')
    }
  }

  // Save the current flow (manual or auto)
  const saveFlow = useCallback(
    async (auto = false) => {
      if (!activeFlow) return
      setSaving(true)
      setError('')
      try {
        const flowData = serializeFlowData(nodesRef.current, edgesRef.current)
        const updated = await updateFlow(activeFlow.id, {
          name: flowName || activeFlow.name,
          flow_data: flowData,
        })
        setActiveFlow(updated)
        if (!auto) setSuccess('Flow saved')
        // Clear success after a moment
        if (!auto) setTimeout(() => setSuccess(''), 2000)
      } catch (err) {
        setError(err instanceof ApiError ? err.message : 'failed to save flow')
      } finally {
        setSaving(false)
      }
    },
    [activeFlow, flowName],
  )

  // Auto-save on changes (debounced)
  const handleFlowChange = useCallback(
    (nodes: Node[], edges: Edge[]) => {
      nodesRef.current = nodes
      edgesRef.current = edges
      if (autoSaveTimer.current) clearTimeout(autoSaveTimer.current)
      autoSaveTimer.current = setTimeout(() => saveFlow(true), 2000)
    },
    [saveFlow],
  )

  // Publish
  async function handlePublish() {
    if (!activeFlow) return
    // Save first
    await saveFlow()
    setPublishing(true)
    setError('')
    try {
      const published = await publishFlow(activeFlow.id)
      setActiveFlow(published)
      setSuccess('Flow published successfully')
      setTimeout(() => setSuccess(''), 3000)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'failed to publish flow')
    } finally {
      setPublishing(false)
    }
  }

  // Validate
  async function handleValidate() {
    if (!activeFlow) return
    // Save first
    await saveFlow()
    setValidating(true)
    setError('')
    try {
      const result = await validateFlow(activeFlow.id)
      setValidationResult(result)

      // Map issues to nodes
      const issuesByNode: Record<string, { severity: 'error' | 'warning'; message: string }[]> = {}
      for (const issue of result.issues) {
        if (issue.node_id) {
          if (!issuesByNode[issue.node_id]) issuesByNode[issue.node_id] = []
          issuesByNode[issue.node_id].push({
            severity: issue.severity,
            message: issue.message,
          })
        }
      }
      setValidationIssues(issuesByNode)

      if (result.valid) {
        setSuccess('Flow is valid')
        setTimeout(() => setSuccess(''), 2000)
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'failed to validate flow')
    } finally {
      setValidating(false)
    }
  }

  // Close editor, return to list
  function closeEditor() {
    if (autoSaveTimer.current) clearTimeout(autoSaveTimer.current)
    // Final save before closing
    saveFlow(true)
    setView('list')
    setActiveFlow(null)
    loadFlows()
  }

  // ---------- EDITOR VIEW ----------
  if (view === 'editor' && activeFlow) {
    const { nodes, edges } = parseFlowData(activeFlow.flow_data)

    return (
      <div className="flex flex-col h-[calc(100vh-theme(spacing.14)-theme(spacing.12))]">
        {/* Editor toolbar */}
        <div className="flex items-center justify-between px-4 py-2 bg-white border-b border-gray-200 shrink-0">
          <div className="flex items-center gap-3">
            <button
              type="button"
              onClick={closeEditor}
              className="text-sm text-gray-500 hover:text-gray-700 flex items-center gap-1"
            >
              <svg className="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
                <path fillRule="evenodd" d="M9.707 16.707a1 1 0 01-1.414 0l-6-6a1 1 0 010-1.414l6-6a1 1 0 011.414 1.414L5.414 9H17a1 1 0 110 2H5.414l4.293 4.293a1 1 0 010 1.414z" clipRule="evenodd" />
              </svg>
              Back
            </button>
            <input
              type="text"
              value={flowName}
              onChange={(e) => setFlowName(e.target.value)}
              onBlur={() => saveFlow(true)}
              className="text-lg font-semibold text-gray-900 bg-transparent border-none focus:outline-none focus:ring-0 p-0"
              placeholder="Flow name..."
            />
            {activeFlow.published && (
              <span className="inline-flex items-center rounded-full bg-green-50 px-2 py-0.5 text-xs font-medium text-green-700">
                Published v{activeFlow.version}
              </span>
            )}
            {saving && (
              <span className="text-xs text-gray-400">Saving...</span>
            )}
          </div>

          <div className="flex items-center gap-2">
            {error && (
              <span className="text-xs text-red-600">{error}</span>
            )}
            {success && (
              <span className="text-xs text-green-600">{success}</span>
            )}
            <button
              type="button"
              onClick={() => saveFlow()}
              disabled={saving}
              className="rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50 transition-colors"
            >
              Save
            </button>
            <button
              type="button"
              onClick={handleValidate}
              disabled={validating}
              className="rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50 transition-colors"
            >
              {validating ? 'Validating...' : 'Validate'}
            </button>
            <button
              type="button"
              onClick={handlePublish}
              disabled={publishing}
              className="rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50 transition-colors"
            >
              {publishing ? 'Publishing...' : 'Publish'}
            </button>
          </div>
        </div>

        {/* Validation results banner */}
        {validationResult && !validationResult.valid && (
          <div className="bg-red-50 border-b border-red-200 px-4 py-2 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <svg className="w-4 h-4 text-red-500" viewBox="0 0 20 20" fill="currentColor">
                <path fillRule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clipRule="evenodd" />
              </svg>
              <span className="text-sm text-red-700">
                {validationResult.issues.length} issue{validationResult.issues.length !== 1 ? 's' : ''} found
              </span>
            </div>
            <button
              type="button"
              onClick={() => { setValidationResult(null); setValidationIssues({}) }}
              className="text-xs text-red-500 hover:text-red-700"
            >
              Dismiss
            </button>
          </div>
        )}

        {/* Canvas */}
        <FlowCanvas
          initialNodes={nodes}
          initialEdges={edges}
          onChange={handleFlowChange}
          validationIssues={Object.keys(validationIssues).length > 0 ? validationIssues : undefined}
        />
      </div>
    )
  }

  // ---------- LIST VIEW ----------
  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Call Flows</h1>
          <p className="mt-1 text-sm text-gray-500">
            Visual call flow editor. Create and manage call routing logic.
          </p>
        </div>
        <button
          type="button"
          onClick={handleCreate}
          className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 transition-colors"
        >
          New Flow
        </button>
      </div>

      {error && (
        <div className="rounded-md bg-red-50 border border-red-200 px-3 py-2 mb-4">
          <p className="text-sm text-red-700">{error}</p>
        </div>
      )}

      {loading ? (
        <p className="text-sm text-gray-400">Loading...</p>
      ) : flows.length === 0 ? (
        <div className="text-center py-12">
          <svg className="mx-auto w-12 h-12 text-gray-300" viewBox="0 0 20 20" fill="currentColor">
            <path fillRule="evenodd" d="M6 2a2 2 0 00-2 2v1a2 2 0 002 2h1v2H5a2 2 0 00-2 2v1a2 2 0 002 2h1v2a2 2 0 002 2h4a2 2 0 002-2v-2h1a2 2 0 002-2v-1a2 2 0 00-2-2h-2V7h1a2 2 0 002-2V4a2 2 0 00-2-2H6zm0 2h8v1H6V4zm-1 7h10v1H5v-1zm3 5h4v1H8v-1z" clipRule="evenodd" />
          </svg>
          <h3 className="mt-4 text-sm font-medium text-gray-900">No call flows</h3>
          <p className="mt-1 text-sm text-gray-500">
            Get started by creating your first call flow.
          </p>
          <button
            type="button"
            onClick={handleCreate}
            className="mt-4 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 transition-colors"
          >
            New Flow
          </button>
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {flows.map((flow) => (
            <div
              key={flow.id}
              onClick={() => openFlow(flow)}
              className="bg-white rounded-lg border border-gray-200 p-4 hover:border-blue-300 hover:shadow-sm cursor-pointer transition-all"
            >
              <div className="flex items-start justify-between">
                <div className="min-w-0 flex-1">
                  <h3 className="text-sm font-semibold text-gray-900 truncate">{flow.name}</h3>
                  <p className="mt-1 text-xs text-gray-500">
                    Version {flow.version}
                    {flow.published_at &&
                      ` \u00b7 Published ${new Date(flow.published_at).toLocaleDateString()}`}
                  </p>
                </div>
                <div className="flex items-center gap-2 ml-2">
                  {flow.published ? (
                    <span className="inline-flex items-center rounded-full bg-green-50 px-2 py-0.5 text-xs font-medium text-green-700">
                      Live
                    </span>
                  ) : (
                    <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-500">
                      Draft
                    </span>
                  )}
                </div>
              </div>
              <div className="mt-3 flex items-center justify-between">
                <span className="text-xs text-gray-400">
                  Updated {new Date(flow.updated_at).toLocaleDateString()}
                </span>
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation()
                    handleDelete(flow)
                  }}
                  className="text-xs text-red-500 hover:text-red-700"
                >
                  Delete
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
