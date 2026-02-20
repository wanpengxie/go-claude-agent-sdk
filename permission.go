package claude

import "context"

// PermissionMode controls tool execution permissions.
type PermissionMode string

const (
	PermissionDefault           PermissionMode = "default"
	PermissionAcceptEdits       PermissionMode = "acceptEdits"
	PermissionPlan              PermissionMode = "plan"
	PermissionBypassPermissions PermissionMode = "bypassPermissions"
)

// PermissionBehavior represents the behavior of a permission rule.
type PermissionBehavior string

const (
	PermissionBehaviorAllow PermissionBehavior = "allow"
	PermissionBehaviorDeny  PermissionBehavior = "deny"
	PermissionBehaviorAsk   PermissionBehavior = "ask"
)

// PermissionUpdateDestination represents where to apply a permission update.
type PermissionUpdateDestination string

const (
	PermissionDestUserSettings    PermissionUpdateDestination = "userSettings"
	PermissionDestProjectSettings PermissionUpdateDestination = "projectSettings"
	PermissionDestLocalSettings   PermissionUpdateDestination = "localSettings"
	PermissionDestSession         PermissionUpdateDestination = "session"
)

// PermissionUpdateType represents the type of permission update.
type PermissionUpdateType string

const (
	PermissionUpdateAddRules          PermissionUpdateType = "addRules"
	PermissionUpdateReplaceRules      PermissionUpdateType = "replaceRules"
	PermissionUpdateRemoveRules       PermissionUpdateType = "removeRules"
	PermissionUpdateSetMode           PermissionUpdateType = "setMode"
	PermissionUpdateAddDirectories    PermissionUpdateType = "addDirectories"
	PermissionUpdateRemoveDirectories PermissionUpdateType = "removeDirectories"
)

// PermissionRuleValue represents a permission rule value.
type PermissionRuleValue struct {
	ToolName    string `json:"toolName"`
	RuleContent string `json:"ruleContent,omitempty"`
}

// PermissionUpdate represents a permission update configuration.
type PermissionUpdate struct {
	Type        PermissionUpdateType        `json:"type"`
	Rules       []PermissionRuleValue       `json:"rules,omitempty"`
	Behavior    PermissionBehavior          `json:"behavior,omitempty"`
	Mode        PermissionMode              `json:"mode,omitempty"`
	Directories []string                    `json:"directories,omitempty"`
	Destination PermissionUpdateDestination `json:"destination,omitempty"`
}

// ToDict converts PermissionUpdate to dictionary format matching the TypeScript control protocol.
func (p *PermissionUpdate) ToDict() map[string]any {
	result := map[string]any{
		"type": string(p.Type),
	}
	if p.Destination != "" {
		result["destination"] = string(p.Destination)
	}
	switch p.Type {
	case PermissionUpdateAddRules, PermissionUpdateReplaceRules, PermissionUpdateRemoveRules:
		if len(p.Rules) > 0 {
			rules := make([]map[string]any, len(p.Rules))
			for i, rule := range p.Rules {
				rules[i] = map[string]any{
					"toolName":    rule.ToolName,
					"ruleContent": rule.RuleContent,
				}
			}
			result["rules"] = rules
		}
		if p.Behavior != "" {
			result["behavior"] = string(p.Behavior)
		}
	case PermissionUpdateSetMode:
		if p.Mode != "" {
			result["mode"] = string(p.Mode)
		}
	case PermissionUpdateAddDirectories, PermissionUpdateRemoveDirectories:
		if len(p.Directories) > 0 {
			result["directories"] = p.Directories
		}
	}
	return result
}

// ToolPermissionContext provides context information for tool permission callbacks.
type ToolPermissionContext struct {
	Signal      any // Future: abort signal support
	Suggestions []PermissionUpdate
}

// PermissionResult is a sealed interface for permission callback results.
type PermissionResult interface {
	permissionResultType() string
}

// PermissionResultAllow represents an allow permission result.
type PermissionResultAllow struct {
	UpdatedInput       map[string]any     `json:"updatedInput,omitempty"`
	UpdatedPermissions []PermissionUpdate `json:"updatedPermissions,omitempty"`
}

func (r *PermissionResultAllow) permissionResultType() string { return "allow" }

// PermissionResultDeny represents a deny permission result.
type PermissionResultDeny struct {
	Message   string `json:"message,omitempty"`
	Interrupt bool   `json:"interrupt,omitempty"`
}

func (r *PermissionResultDeny) permissionResultType() string { return "deny" }

// CanUseToolFunc is the signature for tool permission callbacks.
type CanUseToolFunc func(
	ctx context.Context,
	toolName string,
	input map[string]any,
	permCtx ToolPermissionContext,
) (PermissionResult, error)
