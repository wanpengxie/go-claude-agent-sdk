package claude

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// queryOptions holds configuration for the query handler.
type queryOptions struct {
	CanUseTool        CanUseToolFunc
	Hooks             map[string][]hookMatcherConfig
	SdkMcpServers     map[string]*McpServer
	InitializeTimeout float64
	Agents            map[string]map[string]any
}

// hookMatcherConfig is the internal representation of hook matchers.
type hookMatcherConfig struct {
	Matcher string
	Hooks   []HookCallback
	Timeout *float64
}

// pendingRequest represents a pending control request waiting for response.
type pendingRequest struct {
	done   chan struct{}
	result map[string]any
	err    error
}

// queryHandler handles bidirectional control protocol on top of the transport.
type queryHandler struct {
	transport interface {
		Write(data string) error
		Messages() <-chan map[string]any
		Errors() <-chan error
		LastError() error
		Close() error
		EndInput() error
		IsReady() bool
	}

	canUseTool    CanUseToolFunc
	hooks         map[string][]hookMatcherConfig
	sdkMcpServers map[string]*McpServer
	agents        map[string]map[string]any

	// Control protocol state
	pendingRequests sync.Map // map[string]*pendingRequest
	hookCallbacks   map[string]HookCallback
	nextCallbackID  int
	requestCounter  atomic.Int64

	// Message stream
	msgChan chan map[string]any
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	closed  atomic.Bool

	writeMu sync.Mutex

	// Track first result for proper stream closure
	firstResultOnce sync.Once
	firstResultChan chan struct{}

	streamCloseTimeout float64
	initializeTimeout  float64

	// Initialize result
	initResult map[string]any

	readErr   error
	readErrMu sync.Mutex
}

func newQueryHandler(transport interface {
	Write(data string) error
	Messages() <-chan map[string]any
	Errors() <-chan error
	LastError() error
	Close() error
	EndInput() error
	IsReady() bool
}, opts queryOptions) *queryHandler {
	timeout := opts.InitializeTimeout
	if timeout <= 0 {
		timeout = 60.0
	}

	streamCloseTimeout := 60.0
	if envVal := os.Getenv("CLAUDE_CODE_STREAM_CLOSE_TIMEOUT"); envVal != "" {
		if ms, err := strconv.ParseFloat(envVal, 64); err == nil {
			streamCloseTimeout = ms / 1000.0
		}
	}

	return &queryHandler{
		transport:          transport,
		canUseTool:         opts.CanUseTool,
		hooks:              opts.Hooks,
		sdkMcpServers:      opts.SdkMcpServers,
		agents:             opts.Agents,
		hookCallbacks:      make(map[string]HookCallback),
		msgChan:            make(chan map[string]any, 100),
		firstResultChan:    make(chan struct{}),
		streamCloseTimeout: streamCloseTimeout,
		initializeTimeout:  timeout,
	}
}

func (q *queryHandler) start(ctx context.Context) error {
	readCtx, cancel := context.WithCancel(ctx)
	q.cancel = cancel

	q.wg.Add(1)
	go q.readMessages(readCtx)
	return nil
}

func (q *queryHandler) readMessages(ctx context.Context) {
	defer q.wg.Done()
	defer close(q.msgChan)

	errChan := q.transport.Errors()

	for {
		select {
		case <-ctx.Done():
			if !q.closed.Load() {
				err := ctx.Err()
				if err == nil {
					err = fmt.Errorf("query handler context cancelled")
				}
				q.setReadError(err)
				q.failPendingRequests(err)
				q.pushErrorMessage(context.Background(), err)
			}
			return
		case err, ok := <-errChan:
			if !ok {
				errChan = nil
				continue
			}
			if err != nil {
				q.setReadError(err)
				q.failPendingRequests(err)
				q.pushErrorMessage(ctx, err)
				return
			}
		case msg, ok := <-q.transport.Messages():
			if !ok {
				if err := q.transport.LastError(); err != nil {
					q.setReadError(err)
					q.failPendingRequests(err)
					q.pushErrorMessage(ctx, err)
				}
				return
			}
			if q.closed.Load() {
				return
			}

			msgType, _ := msg["type"].(string)

			switch msgType {
			case "control_response":
				response, _ := msg["response"].(map[string]any)
				if response == nil {
					continue
				}
				requestID, _ := response["request_id"].(string)
				if requestID == "" {
					continue
				}
				if val, ok := q.pendingRequests.Load(requestID); ok {
					pending := val.(*pendingRequest)
					subtype, _ := response["subtype"].(string)
					if subtype == "error" {
						errMsg, _ := response["error"].(string)
						pending.err = fmt.Errorf("%s", errMsg)
					} else {
						pending.result = response
					}
					safeClose(pending.done)
				}

			case "control_request":
				go q.handleControlRequest(ctx, msg)

			case "control_cancel_request":
				// TODO: implement cancellation
				continue

			default:
				// Track result for stream closure
				if msgType == "result" {
					q.firstResultOnce.Do(func() {
						close(q.firstResultChan)
					})
				}
				// Regular SDK message
				select {
				case q.msgChan <- msg:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func (q *queryHandler) handleControlRequest(ctx context.Context, msg map[string]any) {
	requestID, _ := msg["request_id"].(string)
	request, _ := msg["request"].(map[string]any)
	if request == nil || requestID == "" {
		return
	}

	subtype, _ := request["subtype"].(string)
	var responseData map[string]any
	var err error

	switch subtype {
	case "can_use_tool":
		responseData, err = q.handleCanUseTool(ctx, request)
	case "hook_callback":
		responseData, err = q.handleHookCallback(ctx, request)
	case "mcp_message":
		responseData, err = q.handleMcpMessage(ctx, request)
	default:
		err = fmt.Errorf("unsupported control request subtype: %s", subtype)
	}

	var response map[string]any
	if err != nil {
		response = map[string]any{
			"type": "control_response",
			"response": map[string]any{
				"subtype":    "error",
				"request_id": requestID,
				"error":      err.Error(),
			},
		}
	} else {
		response = map[string]any{
			"type": "control_response",
			"response": map[string]any{
				"subtype":    "success",
				"request_id": requestID,
				"response":   responseData,
			},
		}
	}

	data, _ := json.Marshal(response)
	q.writeMu.Lock()
	_ = q.transport.Write(string(data) + "\n")
	q.writeMu.Unlock()
}

func (q *queryHandler) handleCanUseTool(ctx context.Context, request map[string]any) (map[string]any, error) {
	if q.canUseTool == nil {
		return nil, fmt.Errorf("canUseTool callback is not provided")
	}

	toolName, _ := request["tool_name"].(string)
	input, _ := request["input"].(map[string]any)
	if input == nil {
		input = map[string]any{}
	}

	var suggestions []PermissionUpdate
	if rawSuggestions, ok := request["permission_suggestions"].([]any); ok {
		for _, raw := range rawSuggestions {
			if m, ok := raw.(map[string]any); ok {
				suggestions = append(suggestions, parsePermissionUpdate(m))
			}
		}
	}

	permCtx := ToolPermissionContext{
		Suggestions: suggestions,
	}

	result, err := q.canUseTool(ctx, toolName, input, permCtx)
	if err != nil {
		return nil, err
	}

	switch r := result.(type) {
	case *PermissionResultAllow:
		resp := map[string]any{
			"behavior": "allow",
		}
		if r.UpdatedInput != nil {
			resp["updatedInput"] = r.UpdatedInput
		} else {
			resp["updatedInput"] = input
		}
		if r.UpdatedPermissions != nil {
			perms := make([]map[string]any, len(r.UpdatedPermissions))
			for i, p := range r.UpdatedPermissions {
				perms[i] = p.ToDict()
			}
			resp["updatedPermissions"] = perms
		}
		return resp, nil

	case *PermissionResultDeny:
		resp := map[string]any{
			"behavior": "deny",
			"message":  r.Message,
		}
		if r.Interrupt {
			resp["interrupt"] = true
		}
		return resp, nil

	default:
		return nil, fmt.Errorf("unexpected permission result type: %T", result)
	}
}

func (q *queryHandler) handleHookCallback(ctx context.Context, request map[string]any) (map[string]any, error) {
	callbackID, _ := request["callback_id"].(string)
	callback, ok := q.hookCallbacks[callbackID]
	if !ok {
		return nil, fmt.Errorf("no hook callback found for ID: %s", callbackID)
	}

	var hookInput HookInput
	if rawInput, ok := request["input"].(map[string]any); ok {
		hookInput = parseHookInput(rawInput)
	}

	toolUseID, _ := request["tool_use_id"].(string)
	hookCtx := HookContext{}

	output, err := callback(ctx, hookInput, toolUseID, hookCtx)
	if err != nil {
		return nil, err
	}
	if output == nil {
		return map[string]any{}, nil
	}

	return convertHookOutputForCLI(output), nil
}

func (q *queryHandler) handleMcpMessage(ctx context.Context, request map[string]any) (map[string]any, error) {
	serverName, _ := request["server_name"].(string)
	message, _ := request["message"].(map[string]any)

	if serverName == "" || message == nil {
		return nil, fmt.Errorf("missing server_name or message for MCP request")
	}

	server, ok := q.sdkMcpServers[serverName]
	if !ok {
		return map[string]any{
			"mcp_response": map[string]any{
				"jsonrpc": "2.0",
				"id":      message["id"],
				"error": map[string]any{
					"code":    -32601,
					"message": fmt.Sprintf("Server '%s' not found", serverName),
				},
			},
		}, nil
	}

	mcpResponse := server.HandleRequest(ctx, message)
	return map[string]any{"mcp_response": mcpResponse}, nil
}

func (q *queryHandler) sendControlRequest(ctx context.Context, request map[string]any, timeout float64) (map[string]any, error) {
	if err := q.err(); err != nil {
		return nil, err
	}

	// Generate unique request ID
	counter := q.requestCounter.Add(1)
	randBytes := make([]byte, 4)
	rand.Read(randBytes)
	requestID := fmt.Sprintf("req_%d_%s", counter, hex.EncodeToString(randBytes))

	// Create pending request
	pending := &pendingRequest{done: make(chan struct{})}
	q.pendingRequests.Store(requestID, pending)
	defer q.pendingRequests.Delete(requestID)

	// Build and send control request
	controlRequest := map[string]any{
		"type":       "control_request",
		"request_id": requestID,
		"request":    request,
	}
	data, _ := json.Marshal(controlRequest)

	q.writeMu.Lock()
	err := q.transport.Write(string(data) + "\n")
	q.writeMu.Unlock()
	if err != nil {
		return nil, err
	}

	// Wait for response with timeout
	timer := time.NewTimer(time.Duration(timeout * float64(time.Second)))
	defer timer.Stop()

	select {
	case <-pending.done:
		if pending.err != nil {
			return nil, pending.err
		}
		resp, _ := pending.result["response"].(map[string]any)
		if resp == nil {
			resp = map[string]any{}
		}
		return resp, nil
	case <-timer.C:
		subtype, _ := request["subtype"].(string)
		return nil, fmt.Errorf("control request timeout: %s", subtype)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (q *queryHandler) initialize(ctx context.Context) (map[string]any, error) {
	// Build hooks configuration
	hooksConfig := map[string]any{}
	if len(q.hooks) > 0 {
		for event, matchers := range q.hooks {
			if len(matchers) == 0 {
				continue
			}
			var matcherConfigs []map[string]any
			for _, matcher := range matchers {
				callbackIDs := make([]string, len(matcher.Hooks))
				for i, callback := range matcher.Hooks {
					callbackID := fmt.Sprintf("hook_%d", q.nextCallbackID)
					q.nextCallbackID++
					q.hookCallbacks[callbackID] = callback
					callbackIDs[i] = callbackID
				}
				mc := map[string]any{
					"matcher":         matcher.Matcher,
					"hookCallbackIds": callbackIDs,
				}
				if matcher.Timeout != nil {
					mc["timeout"] = *matcher.Timeout
				}
				matcherConfigs = append(matcherConfigs, mc)
			}
			hooksConfig[event] = matcherConfigs
		}
	}

	request := map[string]any{
		"subtype": "initialize",
		"hooks":   hooksConfig,
	}
	if len(q.agents) > 0 {
		request["agents"] = q.agents
	}

	resp, err := q.sendControlRequest(ctx, request, q.initializeTimeout)
	if err != nil {
		return nil, err
	}
	q.initResult = resp
	return resp, nil
}

func (q *queryHandler) interrupt(ctx context.Context) error {
	_, err := q.sendControlRequest(ctx, map[string]any{"subtype": "interrupt"}, 60.0)
	return err
}

func (q *queryHandler) setPermissionMode(ctx context.Context, mode string) error {
	_, err := q.sendControlRequest(ctx, map[string]any{
		"subtype": "set_permission_mode",
		"mode":    mode,
	}, 60.0)
	return err
}

func (q *queryHandler) setModel(ctx context.Context, model string) error {
	modelAny := any(model)
	return q.setModelOptional(ctx, modelAny)
}

func (q *queryHandler) setModelOptional(ctx context.Context, model any) error {
	_, err := q.sendControlRequest(ctx, map[string]any{
		"subtype": "set_model",
		"model":   model,
	}, 60.0)
	return err
}

func (q *queryHandler) rewindFiles(ctx context.Context, userMessageID string) error {
	_, err := q.sendControlRequest(ctx, map[string]any{
		"subtype":         "rewind_files",
		"user_message_id": userMessageID,
	}, 60.0)
	return err
}

func (q *queryHandler) getMcpStatus(ctx context.Context) (map[string]any, error) {
	return q.sendControlRequest(ctx, map[string]any{"subtype": "mcp_status"}, 60.0)
}

func (q *queryHandler) receiveMessages() <-chan map[string]any {
	return q.msgChan
}

func (q *queryHandler) streamInput(ctx context.Context, messages <-chan map[string]any) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-messages:
			if !ok {
				// Input stream ended
				hasHooks := len(q.hooks) > 0
				if len(q.sdkMcpServers) > 0 || hasHooks {
					log.Printf("Waiting for first result before closing stdin (sdk_mcp_servers=%d, has_hooks=%v)",
						len(q.sdkMcpServers), hasHooks)
					select {
					case <-q.firstResultChan:
					case <-time.After(time.Duration(q.streamCloseTimeout * float64(time.Second))):
					case <-ctx.Done():
					}
				}
				_ = q.transport.EndInput()
				return
			}
			if q.closed.Load() {
				return
			}
			data, _ := json.Marshal(msg)
			q.writeMu.Lock()
			_ = q.transport.Write(string(data) + "\n")
			q.writeMu.Unlock()
		}
	}
}

// parsePermissionUpdate converts a raw map to a PermissionUpdate struct.
func parsePermissionUpdate(m map[string]any) PermissionUpdate {
	pu := PermissionUpdate{}
	if v, ok := m["type"].(string); ok {
		pu.Type = PermissionUpdateType(v)
	}
	if v, ok := m["behavior"].(string); ok {
		pu.Behavior = PermissionBehavior(v)
	}
	if v, ok := m["mode"].(string); ok {
		pu.Mode = PermissionMode(v)
	}
	if v, ok := m["destination"].(string); ok {
		pu.Destination = PermissionUpdateDestination(v)
	}
	if v, ok := m["directories"].([]any); ok {
		dirs := make([]string, 0, len(v))
		for _, d := range v {
			if s, ok := d.(string); ok {
				dirs = append(dirs, s)
			}
		}
		pu.Directories = dirs
	}
	if v, ok := m["rules"].([]any); ok {
		rules := make([]PermissionRuleValue, 0, len(v))
		for _, raw := range v {
			if rm, ok := raw.(map[string]any); ok {
				rule := PermissionRuleValue{}
				if tn, ok := rm["toolName"].(string); ok {
					rule.ToolName = tn
				}
				if rc, ok := rm["ruleContent"].(string); ok {
					rule.RuleContent = rc
				}
				rules = append(rules, rule)
			}
		}
		pu.Rules = rules
	}
	return pu
}

// parseHookInput converts a raw map to a HookInput struct without json round-trip.
func parseHookInput(m map[string]any) HookInput {
	input := HookInput{}
	if v, ok := m["session_id"].(string); ok {
		input.SessionID = v
	}
	if v, ok := m["transcript_path"].(string); ok {
		input.TranscriptPath = v
	}
	if v, ok := m["cwd"].(string); ok {
		input.Cwd = v
	}
	if v, ok := m["permission_mode"].(string); ok {
		input.PermissionMode = v
	}
	if v, ok := m["hook_event_name"].(string); ok {
		input.HookEventName = v
	}
	if v, ok := m["tool_name"].(string); ok {
		input.ToolName = v
	}
	if v, ok := m["tool_input"].(map[string]any); ok {
		input.ToolInput = v
	}
	if v, ok := m["tool_use_id"].(string); ok {
		input.ToolUseID = v
	}
	if v := m["tool_response"]; v != nil {
		input.ToolResponse = v
	}
	if v, ok := m["error"].(string); ok {
		input.ErrorMsg = v
	}
	if v, ok := m["is_interrupt"].(bool); ok {
		input.IsInterrupt = &v
	}
	if v, ok := m["prompt"].(string); ok {
		input.Prompt = v
	}
	if v, ok := m["stop_hook_active"].(bool); ok {
		input.StopHookActive = v
	}
	if v, ok := m["agent_id"].(string); ok {
		input.AgentID = v
	}
	if v, ok := m["agent_transcript_path"].(string); ok {
		input.AgentTranscriptPath = v
	}
	if v, ok := m["agent_type"].(string); ok {
		input.AgentType = v
	}
	if v, ok := m["trigger"].(string); ok {
		input.Trigger = v
	}
	if v, ok := m["custom_instructions"].(string); ok {
		input.CustomInstructions = v
	}
	if v, ok := m["message"].(string); ok {
		input.NotificationMessage = v
	}
	if v, ok := m["title"].(string); ok {
		input.Title = v
	}
	if v, ok := m["notification_type"].(string); ok {
		input.NotificationType = v
	}
	if v, ok := m["permission_suggestions"].([]any); ok {
		input.PermissionSuggestions = v
	}
	return input
}

// convertHookOutputForCLI converts a HookJSONOutput to a map for the CLI.
func convertHookOutputForCLI(output *HookJSONOutput) map[string]any {
	result := map[string]any{}
	if output.Async != nil {
		result["async"] = *output.Async
	}
	if output.AsyncTimeout != nil {
		result["asyncTimeout"] = *output.AsyncTimeout
	}
	if output.Continue != nil {
		result["continue"] = *output.Continue
	}
	if output.SuppressOutput != nil {
		result["suppressOutput"] = *output.SuppressOutput
	}
	if output.StopReason != "" {
		result["stopReason"] = output.StopReason
	}
	if output.Decision != "" {
		result["decision"] = output.Decision
	}
	if output.SystemMessage != "" {
		result["systemMessage"] = output.SystemMessage
	}
	if output.Reason != "" {
		result["reason"] = output.Reason
	}
	if output.HookSpecificOutput != nil {
		hso := map[string]any{
			"hookEventName": output.HookSpecificOutput.HookEventName,
		}
		if output.HookSpecificOutput.PermissionDecision != "" {
			hso["permissionDecision"] = output.HookSpecificOutput.PermissionDecision
		}
		if output.HookSpecificOutput.PermissionDecisionReason != "" {
			hso["permissionDecisionReason"] = output.HookSpecificOutput.PermissionDecisionReason
		}
		if output.HookSpecificOutput.UpdatedInput != nil {
			hso["updatedInput"] = output.HookSpecificOutput.UpdatedInput
		}
		if output.HookSpecificOutput.UpdatedMCPToolOutput != nil {
			hso["updatedMCPToolOutput"] = output.HookSpecificOutput.UpdatedMCPToolOutput
		}
		if output.HookSpecificOutput.AdditionalContext != "" {
			hso["additionalContext"] = output.HookSpecificOutput.AdditionalContext
		}
		if output.HookSpecificOutput.Decision != nil {
			hso["decision"] = output.HookSpecificOutput.Decision
		}
		result["hookSpecificOutput"] = hso
	}
	return result
}

func (q *queryHandler) close() {
	q.closed.Store(true)
	q.failPendingRequests(fmt.Errorf("query handler closed"))
	if q.cancel != nil {
		q.cancel()
	}
	q.wg.Wait()
	_ = q.transport.Close()
}

func (q *queryHandler) pushErrorMessage(ctx context.Context, err error) {
	if err == nil {
		return
	}
	select {
	case q.msgChan <- map[string]any{
		"type":  "error",
		"error": err.Error(),
	}:
	case <-ctx.Done():
	}
}

func (q *queryHandler) setReadError(err error) {
	if err == nil {
		return
	}
	q.readErrMu.Lock()
	defer q.readErrMu.Unlock()
	if q.readErr == nil {
		q.readErr = err
	}
}

func (q *queryHandler) err() error {
	q.readErrMu.Lock()
	defer q.readErrMu.Unlock()
	return q.readErr
}

func (q *queryHandler) failPendingRequests(err error) {
	if err == nil {
		return
	}
	q.pendingRequests.Range(func(_, value any) bool {
		pending, ok := value.(*pendingRequest)
		if !ok {
			return true
		}
		if pending.err == nil {
			pending.err = err
		}
		safeClose(pending.done)
		return true
	})
}

func safeClose(ch chan struct{}) {
	defer func() { _ = recover() }()
	close(ch)
}
