package claude

import (
	"testing"
)

func TestParseUserMessageString(t *testing.T) {
	data := map[string]any{
		"type": "user",
		"message": map[string]any{
			"content": "Hello, world!",
		},
	}
	msg, err := parseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	um, ok := msg.(*UserMessage)
	if !ok {
		t.Fatalf("expected *UserMessage, got %T", msg)
	}
	if um.Content != "Hello, world!" {
		t.Errorf("expected content 'Hello, world!', got %v", um.Content)
	}
}

func TestParseUserMessageWithContentBlocks(t *testing.T) {
	data := map[string]any{
		"type": "user",
		"uuid": "test-uuid",
		"message": map[string]any{
			"content": []any{
				map[string]any{"type": "text", "text": "Hello"},
				map[string]any{
					"type":        "tool_result",
					"tool_use_id": "tool-1",
					"content":     "result text",
				},
			},
		},
		"parent_tool_use_id": "parent-1",
	}
	msg, err := parseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	um, ok := msg.(*UserMessage)
	if !ok {
		t.Fatalf("expected *UserMessage, got %T", msg)
	}
	blocks, ok := um.Content.([]ContentBlock)
	if !ok {
		t.Fatalf("expected []ContentBlock, got %T", um.Content)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if tb, ok := blocks[0].(*TextBlock); ok {
		if tb.Text != "Hello" {
			t.Errorf("expected 'Hello', got %s", tb.Text)
		}
	} else {
		t.Errorf("expected *TextBlock, got %T", blocks[0])
	}
	if um.UUID != "test-uuid" {
		t.Errorf("expected uuid 'test-uuid', got %s", um.UUID)
	}
	if um.ParentToolUseID != "parent-1" {
		t.Errorf("expected parent_tool_use_id 'parent-1', got %s", um.ParentToolUseID)
	}
}

func TestParseAssistantMessage(t *testing.T) {
	data := map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"model": "claude-sonnet-4-5",
			"content": []any{
				map[string]any{"type": "text", "text": "Hello!"},
				map[string]any{
					"type":      "thinking",
					"thinking":  "Let me think...",
					"signature": "sig123",
				},
				map[string]any{
					"type":  "tool_use",
					"id":    "tool-1",
					"name":  "Read",
					"input": map[string]any{"path": "/tmp/test"},
				},
			},
		},
	}
	msg, err := parseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	am, ok := msg.(*AssistantMessage)
	if !ok {
		t.Fatalf("expected *AssistantMessage, got %T", msg)
	}
	if am.Model != "claude-sonnet-4-5" {
		t.Errorf("expected model 'claude-sonnet-4-5', got %s", am.Model)
	}
	if len(am.Content) != 3 {
		t.Fatalf("expected 3 content blocks, got %d", len(am.Content))
	}

	if tb, ok := am.Content[0].(*TextBlock); ok {
		if tb.Text != "Hello!" {
			t.Errorf("expected 'Hello!', got %s", tb.Text)
		}
	} else {
		t.Errorf("expected *TextBlock, got %T", am.Content[0])
	}

	if th, ok := am.Content[1].(*ThinkingBlock); ok {
		if th.Thinking != "Let me think..." {
			t.Errorf("expected 'Let me think...', got %s", th.Thinking)
		}
		if th.Signature != "sig123" {
			t.Errorf("expected signature 'sig123', got %s", th.Signature)
		}
	} else {
		t.Errorf("expected *ThinkingBlock, got %T", am.Content[1])
	}

	if tu, ok := am.Content[2].(*ToolUseBlock); ok {
		if tu.Name != "Read" {
			t.Errorf("expected tool name 'Read', got %s", tu.Name)
		}
	} else {
		t.Errorf("expected *ToolUseBlock, got %T", am.Content[2])
	}
}

func TestParseSystemMessage(t *testing.T) {
	data := map[string]any{
		"type":    "system",
		"subtype": "init",
	}
	msg, err := parseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sm, ok := msg.(*SystemMessage)
	if !ok {
		t.Fatalf("expected *SystemMessage, got %T", msg)
	}
	if sm.Subtype != "init" {
		t.Errorf("expected subtype 'init', got %s", sm.Subtype)
	}
}

func TestParseResultMessage(t *testing.T) {
	data := map[string]any{
		"type":            "result",
		"subtype":         "success",
		"duration_ms":     float64(1000),
		"duration_api_ms": float64(800),
		"is_error":        false,
		"num_turns":       float64(3),
		"session_id":      "sess-123",
		"total_cost_usd":  0.05,
		"result":          "The answer is 4.",
	}
	msg, err := parseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rm, ok := msg.(*ResultMessage)
	if !ok {
		t.Fatalf("expected *ResultMessage, got %T", msg)
	}
	if rm.DurationMS != 1000 {
		t.Errorf("expected duration_ms 1000, got %d", rm.DurationMS)
	}
	if rm.NumTurns != 3 {
		t.Errorf("expected num_turns 3, got %d", rm.NumTurns)
	}
	if rm.TotalCostUSD == nil || *rm.TotalCostUSD != 0.05 {
		t.Errorf("expected total_cost_usd 0.05, got %v", rm.TotalCostUSD)
	}
	if rm.Result != "The answer is 4." {
		t.Errorf("expected result 'The answer is 4.', got %s", rm.Result)
	}
}

func TestParseStreamEvent(t *testing.T) {
	data := map[string]any{
		"type":       "stream_event",
		"uuid":       "uuid-1",
		"session_id": "sess-1",
		"event":      map[string]any{"type": "content_block_delta"},
	}
	msg, err := parseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	se, ok := msg.(*StreamEvent)
	if !ok {
		t.Fatalf("expected *StreamEvent, got %T", msg)
	}
	if se.UUID != "uuid-1" {
		t.Errorf("expected uuid 'uuid-1', got %s", se.UUID)
	}
}

func TestParseRateLimitEvent(t *testing.T) {
	data := map[string]any{
		"type":      "rate_limit_event",
		"remaining": float64(10),
	}
	msg, err := parseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rl, ok := msg.(*RateLimitEvent)
	if !ok {
		t.Fatalf("expected *RateLimitEvent, got %T", msg)
	}
	if typ, _ := rl.Data["type"].(string); typ != "rate_limit_event" {
		t.Fatalf("expected type rate_limit_event in payload, got %q", typ)
	}
}

func TestParseMissingType(t *testing.T) {
	data := map[string]any{"foo": "bar"}
	_, err := parseMessage(data)
	if err == nil {
		t.Fatal("expected error for missing type")
	}
}

func TestParseUnknownType(t *testing.T) {
	data := map[string]any{"type": "unknown_type"}
	_, err := parseMessage(data)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}
