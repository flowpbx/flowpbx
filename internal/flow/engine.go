package flow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/emiago/sipgo/sip"
	"github.com/flowpbx/flowpbx/internal/database"
	"github.com/flowpbx/flowpbx/internal/database/models"
)

// Default timeout per node if none is specified in node config.
const defaultNodeTimeout = 30 * time.Second

// ErrFlowNotFound is returned when the specified flow does not exist.
var ErrFlowNotFound = errors.New("flow not found")

// ErrFlowNotPublished is returned when the flow has not been published.
var ErrFlowNotPublished = errors.New("flow not published")

// ErrEntryNodeNotFound is returned when the entry node cannot be found in the graph.
var ErrEntryNodeNotFound = errors.New("entry node not found in flow")

// ErrNoMatchingEdge is returned when no output edge matches after node execution.
var ErrNoMatchingEdge = errors.New("no matching output edge")

// ErrNodeHandlerNotFound is returned when no handler is registered for a node type.
var ErrNodeHandlerNotFound = errors.New("no handler registered for node type")

// Node represents a single node in the flow graph, parsed from the React Flow JSON.
type Node struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Position Position `json:"position"`
	Data     NodeData `json:"data"`
}

// Position holds the x/y canvas position of a node (for editor persistence).
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// NodeData holds the configuration data for a flow node.
type NodeData struct {
	Label      string         `json:"label"`
	EntityID   *int64         `json:"entity_id,omitempty"`
	EntityType string         `json:"entity_type,omitempty"`
	Config     map[string]any `json:"config,omitempty"`
}

// Edge represents a connection between two nodes in the flow graph.
type Edge struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	Target       string `json:"target"`
	SourceHandle string `json:"sourceHandle"`
	TargetHandle string `json:"targetHandle"`
	Label        string `json:"label,omitempty"`
}

// FlowGraph is the parsed representation of the React Flow JSON stored in call_flows.flow_data.
type FlowGraph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// NodeHandler is the interface that all flow node types must implement.
// Execute processes the node's logic and returns the name of the output
// edge handle to follow (e.g. "next", "timeout", "default", "1", "2"),
// or an error if execution fails.
type NodeHandler interface {
	Execute(ctx context.Context, callCtx *CallContext, node Node) (outputEdge string, err error)
}

// EntityResolver loads a database entity by ID and type. This allows node
// handlers to look up the entity (extension, ring group, voicemail box, etc.)
// referenced by the node's entity_id/entity_type fields.
type EntityResolver interface {
	ResolveEntity(ctx context.Context, entityType string, entityID int64) (any, error)
}

// Engine is the flow graph walker. It loads a published flow, resolves the
// entry node, and walks the graph by executing nodes and following edges.
type Engine struct {
	flows    database.CallFlowRepository
	cdrs     database.CDRRepository
	handlers map[string]NodeHandler
	resolver EntityResolver
	logger   *slog.Logger
}

// NewEngine creates a new flow engine.
func NewEngine(
	flows database.CallFlowRepository,
	cdrs database.CDRRepository,
	resolver EntityResolver,
	logger *slog.Logger,
) *Engine {
	return &Engine{
		flows:    flows,
		cdrs:     cdrs,
		handlers: make(map[string]NodeHandler),
		resolver: resolver,
		logger:   logger.With("subsystem", "flow_engine"),
	}
}

// RegisterHandler registers a node handler for a specific node type.
func (e *Engine) RegisterHandler(nodeType string, handler NodeHandler) {
	e.handlers[nodeType] = handler
}

// ExecuteFlow loads the published flow, finds the entry node, and walks the graph.
// It updates the CDR with the flow traversal path when complete.
func (e *Engine) ExecuteFlow(callCtx *CallContext, flowID int64, entryNodeID string) error {
	ctx := context.Background()

	// Load the published flow.
	flow, err := e.flows.GetPublished(ctx, flowID)
	if err != nil {
		return fmt.Errorf("loading flow: %w", err)
	}
	if flow == nil {
		return ErrFlowNotFound
	}
	if !flow.Published {
		return ErrFlowNotPublished
	}

	// Parse the flow graph JSON.
	graph, err := ParseFlowGraph(flow.FlowData)
	if err != nil {
		return fmt.Errorf("parsing flow graph: %w", err)
	}

	// Build lookup maps for fast access.
	nodeMap := make(map[string]Node, len(graph.Nodes))
	for _, n := range graph.Nodes {
		nodeMap[n.ID] = n
	}

	// Find the entry node.
	entryNode, ok := nodeMap[entryNodeID]
	if !ok {
		return ErrEntryNodeNotFound
	}

	e.logger.Info("starting flow execution",
		"call_id", callCtx.CallID,
		"flow_id", flowID,
		"entry_node", entryNodeID,
	)

	// Walk the graph starting from the entry node.
	err = e.walkGraph(ctx, callCtx, entryNode, nodeMap, graph.Edges)

	// Record the flow path in the CDR regardless of success/failure.
	e.updateCDRFlowPath(callCtx)

	if err != nil {
		e.logger.Error("flow execution failed",
			"call_id", callCtx.CallID,
			"flow_id", flowID,
			"error", err,
		)
		// Attempt graceful call termination.
		e.terminateCall(callCtx, 500, "Internal Server Error")
		return err
	}

	e.logger.Info("flow execution completed",
		"call_id", callCtx.CallID,
		"flow_id", flowID,
		"nodes_visited", len(callCtx.GetFlowPath()),
	)

	return nil
}

// walkGraph executes nodes sequentially, following edges after each execution.
func (e *Engine) walkGraph(ctx context.Context, callCtx *CallContext, currentNode Node, nodeMap map[string]Node, edges []Edge) error {
	for {
		// Record this node in the traversal path.
		callCtx.RecordNode(currentNode.ID)

		e.logger.Debug("executing node",
			"call_id", callCtx.CallID,
			"node_id", currentNode.ID,
			"node_type", currentNode.Type,
			"label", currentNode.Data.Label,
		)

		// Look up the handler for this node type.
		handler, ok := e.handlers[currentNode.Type]
		if !ok {
			return fmt.Errorf("%w: %s", ErrNodeHandlerNotFound, currentNode.Type)
		}

		// Determine the timeout for this node.
		timeout := e.nodeTimeout(currentNode)
		nodeCtx, cancel := context.WithTimeout(ctx, timeout)

		// Execute the node handler.
		outputEdge, err := handler.Execute(nodeCtx, callCtx, currentNode)
		cancel()

		if err != nil {
			// Check if the error was a timeout.
			if errors.Is(err, context.DeadlineExceeded) {
				e.logger.Warn("node execution timed out",
					"call_id", callCtx.CallID,
					"node_id", currentNode.ID,
					"node_type", currentNode.Type,
					"timeout", timeout,
				)
				// Try the "timeout" output edge.
				outputEdge = "timeout"
			} else {
				return fmt.Errorf("executing node %s (%s): %w", currentNode.ID, currentNode.Type, err)
			}
		}

		// If the output edge is empty, the node is a terminal node (e.g. hangup).
		if outputEdge == "" {
			e.logger.Debug("node returned empty output edge (terminal)",
				"call_id", callCtx.CallID,
				"node_id", currentNode.ID,
			)
			return nil
		}

		// Find the matching edge from this node with the given sourceHandle.
		nextNodeID, err := e.followEdge(currentNode.ID, outputEdge, edges)
		if err != nil {
			return fmt.Errorf("following edge from node %s handle %q: %w", currentNode.ID, outputEdge, err)
		}

		// Resolve the next node.
		nextNode, ok := nodeMap[nextNodeID]
		if !ok {
			return fmt.Errorf("target node %s not found in graph", nextNodeID)
		}

		currentNode = nextNode
	}
}

// followEdge finds the target node ID by matching the source node ID and source handle name.
func (e *Engine) followEdge(sourceNodeID, sourceHandle string, edges []Edge) (string, error) {
	for _, edge := range edges {
		if edge.Source == sourceNodeID && edge.SourceHandle == sourceHandle {
			return edge.Target, nil
		}
	}
	return "", fmt.Errorf("%w: source=%s handle=%s", ErrNoMatchingEdge, sourceNodeID, sourceHandle)
}

// nodeTimeout returns the timeout duration for a node. If the node's config
// specifies a "timeout" value (in seconds), that is used; otherwise the default.
func (e *Engine) nodeTimeout(node Node) time.Duration {
	if node.Data.Config != nil {
		if v, ok := node.Data.Config["timeout"]; ok {
			switch t := v.(type) {
			case float64:
				if t > 0 {
					return time.Duration(t) * time.Second
				}
			case int:
				if t > 0 {
					return time.Duration(t) * time.Second
				}
			}
		}
	}
	return defaultNodeTimeout
}

// updateCDRFlowPath persists the flow traversal path to the CDR record.
func (e *Engine) updateCDRFlowPath(callCtx *CallContext) {
	path := callCtx.GetFlowPath()
	if len(path) == 0 {
		return
	}

	pathJSON, err := json.Marshal(path)
	if err != nil {
		e.logger.Error("failed to marshal flow path",
			"call_id", callCtx.CallID,
			"error", err,
		)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cdr, err := e.cdrs.GetByCallID(ctx, callCtx.CallID)
	if err != nil {
		e.logger.Error("failed to load cdr for flow path update",
			"call_id", callCtx.CallID,
			"error", err,
		)
		return
	}
	if cdr == nil {
		e.logger.Warn("cdr not found for flow path update",
			"call_id", callCtx.CallID,
		)
		return
	}

	cdr.FlowPath = string(pathJSON)
	if err := e.cdrs.Update(ctx, cdr); err != nil {
		e.logger.Error("failed to update cdr flow path",
			"call_id", callCtx.CallID,
			"error", err,
		)
	}
}

// terminateCall attempts to send a SIP error response to gracefully end the call.
func (e *Engine) terminateCall(callCtx *CallContext, code int, reason string) {
	if callCtx.Request == nil || callCtx.Transaction == nil {
		return
	}

	res := sip.NewResponseFromRequest(callCtx.Request, code, reason, nil)
	if err := callCtx.Transaction.Respond(res); err != nil {
		e.logger.Error("failed to send termination response",
			"call_id", callCtx.CallID,
			"code", code,
			"error", err,
		)
	}
}

// ParseFlowGraph parses a React Flow JSON string into a FlowGraph.
func ParseFlowGraph(flowData string) (*FlowGraph, error) {
	var graph FlowGraph
	if err := json.Unmarshal([]byte(flowData), &graph); err != nil {
		return nil, fmt.Errorf("unmarshaling flow graph: %w", err)
	}
	return &graph, nil
}

// ResolveNodeEntity loads the entity referenced by a node's entity_id and entity_type.
// Returns nil if the node has no entity reference.
func (e *Engine) ResolveNodeEntity(ctx context.Context, node Node) (any, error) {
	if node.Data.EntityID == nil || node.Data.EntityType == "" {
		return nil, nil
	}

	if e.resolver == nil {
		return nil, fmt.Errorf("no entity resolver configured")
	}

	return e.resolver.ResolveEntity(ctx, node.Data.EntityType, *node.Data.EntityID)
}

// defaultEntityResolver implements EntityResolver using the database repositories.
type defaultEntityResolver struct {
	extensions     database.ExtensionRepository
	ringGroups     database.RingGroupRepository
	voicemailBoxes database.VoicemailBoxRepository
	ivrMenus       database.IVRMenuRepository
	timeSwitches   database.TimeSwitchRepository
	conferences    database.ConferenceBridgeRepository
	inboundNumbers database.InboundNumberRepository
}

// NewEntityResolver creates a default entity resolver backed by database repositories.
func NewEntityResolver(
	extensions database.ExtensionRepository,
	ringGroups database.RingGroupRepository,
	voicemailBoxes database.VoicemailBoxRepository,
	ivrMenus database.IVRMenuRepository,
	timeSwitches database.TimeSwitchRepository,
	conferences database.ConferenceBridgeRepository,
	inboundNumbers database.InboundNumberRepository,
) EntityResolver {
	return &defaultEntityResolver{
		extensions:     extensions,
		ringGroups:     ringGroups,
		voicemailBoxes: voicemailBoxes,
		ivrMenus:       ivrMenus,
		timeSwitches:   timeSwitches,
		conferences:    conferences,
		inboundNumbers: inboundNumbers,
	}
}

// ResolveEntity loads an entity by type and ID from the appropriate repository.
func (r *defaultEntityResolver) ResolveEntity(ctx context.Context, entityType string, entityID int64) (any, error) {
	switch entityType {
	case "extension":
		return r.extensions.GetByID(ctx, entityID)
	case "ring_group":
		return r.ringGroups.GetByID(ctx, entityID)
	case "voicemail_box":
		return r.voicemailBoxes.GetByID(ctx, entityID)
	case "ivr_menu":
		return r.ivrMenus.GetByID(ctx, entityID)
	case "time_switch":
		return r.timeSwitches.GetByID(ctx, entityID)
	case "conference_bridge":
		return r.conferences.GetByID(ctx, entityID)
	case "inbound_number":
		return r.inboundNumbers.GetByID(ctx, entityID)
	default:
		return nil, fmt.Errorf("unknown entity type: %s", entityType)
	}
}

// Ensure defaultEntityResolver satisfies EntityResolver.
var _ EntityResolver = (*defaultEntityResolver)(nil)

// Ensure Engine satisfies basic contract (compile-time check).
var _ = (*Engine)(nil)

// FlowResult holds the outcome of a flow execution for the caller.
type FlowResult struct {
	// Completed is true if the flow ran to a terminal node without error.
	Completed bool

	// Error is set if the flow failed during execution.
	Error error

	// TargetExtension is set if the flow resolved to routing to an extension.
	TargetExtension *models.Extension

	// FlowPath records the node IDs visited during traversal.
	FlowPath []string
}
