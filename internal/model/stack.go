package model

// StackSummary is a brief overview of a stack.
type StackSummary struct {
	StackName   string `json:"stack_name"`
	StackID     string `json:"stack_id"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at,omitempty"`
	Description string `json:"description,omitempty"`
}

// StackList wraps a list of stack summaries.
type StackList struct {
	Stacks []StackSummary `json:"stacks"`
}

// Resource holds a single CloudFormation stack resource.
type Resource struct {
	LogicalID   string `json:"logical_id"`
	PhysicalID  string `json:"physical_id"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	LastUpdated string `json:"last_updated"`
}

// StackEvent holds a single CloudFormation stack event.
type StackEvent struct {
	Timestamp    string `json:"timestamp"`
	LogicalID    string `json:"logical_id"`
	Status       string `json:"status"`
	StatusReason string `json:"status_reason,omitempty"`
	ResourceType string `json:"resource_type"`
	PhysicalID   string `json:"physical_id,omitempty"`
}

// StackEvents wraps a list of stack events.
type StackEvents struct {
	StackName string       `json:"stack_name"`
	Events    []StackEvent `json:"events"`
}
