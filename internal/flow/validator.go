package flow

import (
	"context"
	"fmt"
)

// ValidationSeverity indicates the severity of a validation issue.
type ValidationSeverity string

const (
	// SeverityError indicates a problem that prevents the flow from working.
	SeverityError ValidationSeverity = "error"
	// SeverityWarning indicates a potential issue that may cause unexpected behavior.
	SeverityWarning ValidationSeverity = "warning"
)

// ValidationIssue describes a single problem found during flow validation.
type ValidationIssue struct {
	Severity ValidationSeverity `json:"severity"`
	NodeID   string             `json:"node_id,omitempty"`
	Message  string             `json:"message"`
}

// ValidationResult holds the outcome of validating a flow graph.
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Issues []ValidationIssue `json:"issues"`
}

// Validator checks a flow graph for structural and referential integrity.
type Validator struct {
	resolver EntityResolver
}

// NewValidator creates a new flow graph validator.
func NewValidator(resolver EntityResolver) *Validator {
	return &Validator{resolver: resolver}
}

// Validate checks the flow graph for common issues:
//   - Nodes with no incoming edges (disconnected, except entry nodes)
//   - Nodes with no outgoing edges (dead ends that aren't terminal types)
//   - Missing entity references (entity_id points to a non-existent record)
//   - Orphan edges (edges referencing non-existent nodes)
//   - Empty graph
func (v *Validator) Validate(ctx context.Context, graph *FlowGraph, entryNodeID string) *ValidationResult {
	result := &ValidationResult{Valid: true, Issues: []ValidationIssue{}}

	if len(graph.Nodes) == 0 {
		result.Valid = false
		result.Issues = append(result.Issues, ValidationIssue{
			Severity: SeverityError,
			Message:  "flow has no nodes",
		})
		return result
	}

	// Build a set of node IDs for quick lookups.
	nodeSet := make(map[string]Node, len(graph.Nodes))
	for _, n := range graph.Nodes {
		nodeSet[n.ID] = n
	}

	// Check that the entry node exists.
	if entryNodeID != "" {
		if _, ok := nodeSet[entryNodeID]; !ok {
			result.Valid = false
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: SeverityError,
				Message:  fmt.Sprintf("entry node %q not found in flow", entryNodeID),
			})
		}
	}

	// Track which nodes have incoming and outgoing edges.
	hasIncoming := make(map[string]bool)
	hasOutgoing := make(map[string]bool)

	// Validate edges.
	for _, edge := range graph.Edges {
		if _, ok := nodeSet[edge.Source]; !ok {
			result.Valid = false
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: SeverityError,
				Message:  fmt.Sprintf("edge %s references non-existent source node %q", edge.ID, edge.Source),
			})
		} else {
			hasOutgoing[edge.Source] = true
		}

		if _, ok := nodeSet[edge.Target]; !ok {
			result.Valid = false
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: SeverityError,
				Message:  fmt.Sprintf("edge %s references non-existent target node %q", edge.ID, edge.Target),
			})
		} else {
			hasIncoming[edge.Target] = true
		}
	}

	// Terminal node types that don't need outgoing edges.
	terminalTypes := map[string]bool{
		"hangup":    true,
		"voicemail": true,
	}

	// Check for disconnected and dead-end nodes.
	for _, node := range graph.Nodes {
		// Entry nodes and inbound_number nodes don't need incoming edges.
		if node.ID != entryNodeID && node.Type != "inbound_number" {
			if !hasIncoming[node.ID] {
				result.Issues = append(result.Issues, ValidationIssue{
					Severity: SeverityWarning,
					NodeID:   node.ID,
					Message:  fmt.Sprintf("node %q (%s) has no incoming edges (disconnected)", node.Data.Label, node.Type),
				})
			}
		}

		// Non-terminal nodes should have at least one outgoing edge.
		if !terminalTypes[node.Type] && !hasOutgoing[node.ID] {
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: SeverityWarning,
				NodeID:   node.ID,
				Message:  fmt.Sprintf("node %q (%s) has no outgoing edges (dead end)", node.Data.Label, node.Type),
			})
		}
	}

	// Validate entity references.
	if v.resolver != nil {
		for _, node := range graph.Nodes {
			if node.Data.EntityID == nil || node.Data.EntityType == "" {
				continue
			}

			entity, err := v.resolver.ResolveEntity(ctx, node.Data.EntityType, *node.Data.EntityID)
			if err != nil {
				result.Valid = false
				result.Issues = append(result.Issues, ValidationIssue{
					Severity: SeverityError,
					NodeID:   node.ID,
					Message:  fmt.Sprintf("node %q: failed to resolve %s with id %d: %v", node.Data.Label, node.Data.EntityType, *node.Data.EntityID, err),
				})
				continue
			}
			if entity == nil {
				result.Valid = false
				result.Issues = append(result.Issues, ValidationIssue{
					Severity: SeverityError,
					NodeID:   node.ID,
					Message:  fmt.Sprintf("node %q: %s with id %d not found", node.Data.Label, node.Data.EntityType, *node.Data.EntityID),
				})
			}
		}
	}

	// If any issues are errors, mark the result as invalid.
	for _, issue := range result.Issues {
		if issue.Severity == SeverityError {
			result.Valid = false
			break
		}
	}

	return result
}
