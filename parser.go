package claude

import "fmt"

// parseMessage converts a raw JSON map from CLI output into a typed Message.
func parseMessage(data map[string]any) (Message, error) {
	msgType, _ := data["type"].(string)
	if msgType == "" {
		return nil, &MessageParseError{
			SDKError: SDKError{Message: "Message missing 'type' field"},
			Data:     data,
		}
	}

	switch msgType {
	case "user":
		return parseUserMessage(data)
	case "assistant":
		return parseAssistantMessage(data)
	case "system":
		return parseSystemMessage(data)
	case "result":
		return parseResultMessage(data)
	case "stream_event":
		return parseStreamEvent(data)
	case "rate_limit_event":
		return parseRateLimitEvent(data)
	default:
		return nil, &MessageParseError{
			SDKError: SDKError{Message: fmt.Sprintf("Unknown message type: %s", msgType)},
			Data:     data,
		}
	}
}

func parseUserMessage(data map[string]any) (*UserMessage, error) {
	msg, ok := data["message"].(map[string]any)
	if !ok {
		return nil, &MessageParseError{
			SDKError: SDKError{Message: "Missing 'message' field in user message"},
			Data:     data,
		}
	}
	content, hasContent := msg["content"]
	if !hasContent {
		return nil, &MessageParseError{
			SDKError: SDKError{Message: "Missing required field in user message: content"},
			Data:     data,
		}
	}

	parentToolUseID, _ := data["parent_tool_use_id"].(string)
	uuid, _ := data["uuid"].(string)
	var toolUseResult map[string]any
	if tur, ok := data["tool_use_result"].(map[string]any); ok {
		toolUseResult = tur
	}

	// If content is a list, parse content blocks
	if contentList, ok := content.([]any); ok {
		blocks := make([]ContentBlock, 0, len(contentList))
		for _, item := range contentList {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			cb := parseContentBlock(block)
			if cb != nil {
				blocks = append(blocks, cb)
			}
		}
		return &UserMessage{
			Content:         blocks,
			UUID:            uuid,
			ParentToolUseID: parentToolUseID,
			ToolUseResult:   toolUseResult,
		}, nil
	}

	// String content
	contentStr, _ := content.(string)
	return &UserMessage{
		Content:         contentStr,
		UUID:            uuid,
		ParentToolUseID: parentToolUseID,
		ToolUseResult:   toolUseResult,
	}, nil
}

func parseAssistantMessage(data map[string]any) (*AssistantMessage, error) {
	msg, ok := data["message"].(map[string]any)
	if !ok {
		return nil, &MessageParseError{
			SDKError: SDKError{Message: "Missing 'message' field in assistant message"},
			Data:     data,
		}
	}

	contentList, ok := msg["content"].([]any)
	if !ok {
		return nil, &MessageParseError{
			SDKError: SDKError{Message: "Missing 'content' field in assistant message"},
			Data:     data,
		}
	}
	model, ok := msg["model"].(string)
	if !ok || model == "" {
		return nil, &MessageParseError{
			SDKError: SDKError{Message: "Missing required field in assistant message: model"},
			Data:     data,
		}
	}

	blocks := make([]ContentBlock, 0, len(contentList))
	for _, item := range contentList {
		block, ok := item.(map[string]any)
		if !ok {
			continue
		}
		cb := parseContentBlock(block)
		if cb != nil {
			blocks = append(blocks, cb)
		}
	}

	parentToolUseID, _ := data["parent_tool_use_id"].(string)
	errorStr, _ := data["error"].(string)

	return &AssistantMessage{
		Content:         blocks,
		Model:           model,
		ParentToolUseID: parentToolUseID,
		Error:           AssistantMessageError(errorStr),
	}, nil
}

func parseSystemMessage(data map[string]any) (*SystemMessage, error) {
	subtype, ok := data["subtype"].(string)
	if !ok {
		return nil, &MessageParseError{
			SDKError: SDKError{Message: "Missing 'subtype' field in system message"},
			Data:     data,
		}
	}
	return &SystemMessage{
		Subtype: subtype,
		Data:    data,
	}, nil
}

func parseResultMessage(data map[string]any) (*ResultMessage, error) {
	subtype, ok := data["subtype"].(string)
	if !ok || subtype == "" {
		return nil, &MessageParseError{
			SDKError: SDKError{Message: "Missing required field in result message: subtype"},
			Data:     data,
		}
	}
	durationRaw, ok := data["duration_ms"]
	if !ok {
		return nil, &MessageParseError{
			SDKError: SDKError{Message: "Missing required field in result message: duration_ms"},
			Data:     data,
		}
	}
	durationAPIRaw, ok := data["duration_api_ms"]
	if !ok {
		return nil, &MessageParseError{
			SDKError: SDKError{Message: "Missing required field in result message: duration_api_ms"},
			Data:     data,
		}
	}
	isError, ok := data["is_error"].(bool)
	if !ok {
		return nil, &MessageParseError{
			SDKError: SDKError{Message: "Missing required field in result message: is_error"},
			Data:     data,
		}
	}
	numTurnsRaw, ok := data["num_turns"]
	if !ok {
		return nil, &MessageParseError{
			SDKError: SDKError{Message: "Missing required field in result message: num_turns"},
			Data:     data,
		}
	}
	sessionID, ok := data["session_id"].(string)
	if !ok || sessionID == "" {
		return nil, &MessageParseError{
			SDKError: SDKError{Message: "Missing required field in result message: session_id"},
			Data:     data,
		}
	}
	durationMS := getIntFromAny(durationRaw)
	durationAPIMS := getIntFromAny(durationAPIRaw)
	numTurns := getIntFromAny(numTurnsRaw)

	rm := &ResultMessage{
		Subtype:       subtype,
		DurationMS:    durationMS,
		DurationAPIMS: durationAPIMS,
		IsError:       isError,
		NumTurns:      numTurns,
		SessionID:     sessionID,
	}

	if cost, ok := data["total_cost_usd"].(float64); ok {
		rm.TotalCostUSD = &cost
	}
	if usage, ok := data["usage"].(map[string]any); ok {
		rm.Usage = usage
	}
	if result, ok := data["result"].(string); ok {
		rm.Result = result
	}
	rm.StructuredOutput = data["structured_output"]

	return rm, nil
}

func parseStreamEvent(data map[string]any) (*StreamEvent, error) {
	uuid, _ := data["uuid"].(string)
	sessionID, _ := data["session_id"].(string)
	event, _ := data["event"].(map[string]any)
	parentToolUseID, _ := data["parent_tool_use_id"].(string)

	if uuid == "" || sessionID == "" || event == nil {
		return nil, &MessageParseError{
			SDKError: SDKError{Message: "Missing required field in stream_event message"},
			Data:     data,
		}
	}

	return &StreamEvent{
		UUID:            uuid,
		SessionID:       sessionID,
		Event:           event,
		ParentToolUseID: parentToolUseID,
	}, nil
}

func parseRateLimitEvent(data map[string]any) (*RateLimitEvent, error) {
	return &RateLimitEvent{Data: data}, nil
}

func parseContentBlock(block map[string]any) ContentBlock {
	blockType, _ := block["type"].(string)
	switch blockType {
	case "text":
		text, _ := block["text"].(string)
		return &TextBlock{Text: text}
	case "thinking":
		thinking, _ := block["thinking"].(string)
		signature, _ := block["signature"].(string)
		return &ThinkingBlock{Thinking: thinking, Signature: signature}
	case "tool_use":
		id, _ := block["id"].(string)
		name, _ := block["name"].(string)
		input, _ := block["input"].(map[string]any)
		return &ToolUseBlock{ID: id, Name: name, Input: input}
	case "tool_result":
		toolUseID, _ := block["tool_use_id"].(string)
		content := block["content"]
		var isError *bool
		if ie, ok := block["is_error"].(bool); ok {
			isError = &ie
		}
		return &ToolResultBlock{ToolUseID: toolUseID, Content: content, IsError: isError}
	default:
		return nil
	}
}

// getIntFromAny converts various numeric types to int.
func getIntFromAny(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}
