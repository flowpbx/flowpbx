package flow

import (
	"sync"
	"time"

	"github.com/emiago/sipgo/sip"
	"github.com/flowpbx/flowpbx/internal/database/models"
)

// CallContext carries all state for a single call traversing a flow graph.
// It is created when an inbound call matches a DID with a flow, and is
// passed to each node handler during execution.
type CallContext struct {
	// CallID is the SIP Call-ID header value.
	CallID string

	// CallerIDName is the display name from the SIP From header.
	CallerIDName string

	// CallerIDNum is the user part of the SIP From URI.
	CallerIDNum string

	// Callee is the dialed number (Request-URI user part).
	Callee string

	// InboundNumber is the matched DID record.
	InboundNumber *models.InboundNumber

	// TrunkID is the inbound trunk that delivered the call.
	TrunkID int64

	// SIP transaction and request for the inbound leg.
	Request     *sip.Request
	Transaction sip.ServerTransaction

	// DTMF holds digits collected during the call (e.g. from an IVR menu).
	DTMF string

	// Variables is a general-purpose key-value store for flow node data.
	Variables map[string]string

	// FlowPath records the ordered list of node IDs visited during traversal.
	FlowPath []string

	// StartTime is when the flow execution began.
	StartTime time.Time

	// mu protects concurrent access to mutable fields (DTMF, Variables, FlowPath).
	mu sync.Mutex
}

// NewCallContext creates a CallContext from the inbound call parameters.
func NewCallContext(
	callID string,
	callerIDName string,
	callerIDNum string,
	callee string,
	inboundNumber *models.InboundNumber,
	trunkID int64,
	req *sip.Request,
	tx sip.ServerTransaction,
) *CallContext {
	return &CallContext{
		CallID:        callID,
		CallerIDName:  callerIDName,
		CallerIDNum:   callerIDNum,
		Callee:        callee,
		InboundNumber: inboundNumber,
		TrunkID:       trunkID,
		Request:       req,
		Transaction:   tx,
		Variables:     make(map[string]string),
		FlowPath:      make([]string, 0),
		StartTime:     time.Now(),
	}
}

// AppendDTMF appends a digit to the collected DTMF buffer.
func (c *CallContext) AppendDTMF(digit string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.DTMF += digit
}

// ClearDTMF resets the collected DTMF buffer.
func (c *CallContext) ClearDTMF() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.DTMF = ""
}

// GetDTMF returns the current DTMF buffer.
func (c *CallContext) GetDTMF() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.DTMF
}

// SetVariable stores a key-value pair in the call context.
func (c *CallContext) SetVariable(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Variables[key] = value
}

// GetVariable retrieves a variable by key. Returns empty string if not found.
func (c *CallContext) GetVariable(key string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Variables[key]
}

// RecordNode appends a node ID to the traversal path.
func (c *CallContext) RecordNode(nodeID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.FlowPath = append(c.FlowPath, nodeID)
}

// GetFlowPath returns a copy of the traversal path.
func (c *CallContext) GetFlowPath() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	path := make([]string, len(c.FlowPath))
	copy(path, c.FlowPath)
	return path
}
