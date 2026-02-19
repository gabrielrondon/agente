package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const defaultModel = anthropic.ModelClaudeSonnet4_5_20250929

// Client wraps the Anthropic SDK.
type Client struct {
	inner anthropic.Client // value, not pointer
	model anthropic.Model
}

// New creates a Claude client using ANTHROPIC_API_KEY from env.
func New() (*Client, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}
	c := anthropic.NewClient(option.WithAPIKey(key))
	return &Client{inner: c, model: defaultModel}, nil
}

// ToolDef defines a Claude tool.
type ToolDef struct {
	Name        string
	Description string
	InputSchema any
}

// ChatRequest is a single-turn request with optional tools.
type ChatRequest struct {
	System string
	User   string
	Tools  []ToolDef
}

// Chat sends a message and returns Claude's text response.
func (c *Client) Chat(ctx context.Context, req ChatRequest) (string, error) {
	params := anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: 4096,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(req.User)),
		},
	}

	if req.System != "" {
		params.System = []anthropic.TextBlockParam{{Text: req.System}}
	}

	if len(req.Tools) > 0 {
		params.Tools = buildTools(req.Tools)
	}

	msg, err := c.inner.Messages.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("claude api: %w", err)
	}

	var text string
	for _, block := range msg.Content {
		switch b := block.AsAny().(type) {
		case anthropic.TextBlock:
			text += b.Text
		}
	}
	return text, nil
}

// ChatWithTools performs an agentic loop: calls Claude, executes tools via executor,
// feeds results back, until stop_reason is end_turn or there are no tool calls.
func (c *Client) ChatWithTools(
	ctx context.Context,
	req ChatRequest,
	executor func(name string, input json.RawMessage) (string, error),
) (string, error) {
	params := anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: 4096,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(req.User)),
		},
	}

	if req.System != "" {
		params.System = []anthropic.TextBlockParam{{Text: req.System}}
	}

	if len(req.Tools) > 0 {
		params.Tools = buildTools(req.Tools)
	}

	for {
		msg, err := c.inner.Messages.New(ctx, params)
		if err != nil {
			return "", fmt.Errorf("claude api: %w", err)
		}

		var text string
		var toolCalls []anthropic.ToolUseBlock

		for _, block := range msg.Content {
			switch b := block.AsAny().(type) {
			case anthropic.TextBlock:
				text += b.Text
			case anthropic.ToolUseBlock:
				toolCalls = append(toolCalls, b)
			}
		}

		if msg.StopReason == "end_turn" || len(toolCalls) == 0 {
			return text, nil
		}

		// Build assistant turn from the response content
		assistantBlocks := make([]anthropic.ContentBlockParamUnion, len(msg.Content))
		for i, block := range msg.Content {
			switch b := block.AsAny().(type) {
			case anthropic.TextBlock:
				assistantBlocks[i] = anthropic.NewTextBlock(b.Text)
			case anthropic.ToolUseBlock:
				assistantBlocks[i] = anthropic.NewToolUseBlock(b.ID, b.Input, b.Name)
			}
		}
		params.Messages = append(params.Messages, anthropic.NewAssistantMessage(assistantBlocks...))

		// Execute tools and collect results
		toolResults := make([]anthropic.ContentBlockParamUnion, 0, len(toolCalls))
		for _, tc := range toolCalls {
			inputRaw, _ := json.Marshal(tc.Input)
			result, err := executor(tc.Name, inputRaw)
			isError := err != nil
			if isError {
				result = fmt.Sprintf("error: %s", err)
			}
			toolResults = append(toolResults, anthropic.NewToolResultBlock(tc.ID, result, isError))
		}
		params.Messages = append(params.Messages, anthropic.NewUserMessage(toolResults...))
	}
}

func buildTools(defs []ToolDef) []anthropic.ToolUnionParam {
	tools := make([]anthropic.ToolUnionParam, len(defs))
	for i, t := range defs {
		schema := toInputSchema(t.InputSchema)
		tools[i] = anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        t.Name,
				Description: anthropic.String(t.Description),
				InputSchema: schema,
			},
		}
	}
	return tools
}

func toInputSchema(v any) anthropic.ToolInputSchemaParam {
	if v == nil {
		return anthropic.ToolInputSchemaParam{Properties: map[string]interface{}{}}
	}

	// Marshal then unmarshal to normalise to map
	b, err := json.Marshal(v)
	if err != nil {
		return anthropic.ToolInputSchemaParam{Properties: map[string]interface{}{}}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return anthropic.ToolInputSchemaParam{Properties: map[string]interface{}{}}
	}

	props, _ := m["properties"].(map[string]any)
	var required []string
	if r, ok := m["required"].([]any); ok {
		for _, s := range r {
			if sv, ok := s.(string); ok {
				required = append(required, sv)
			}
		}
	}

	return anthropic.ToolInputSchemaParam{
		Properties: props,
		Required:   required,
	}
}
