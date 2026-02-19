package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
)

const (
	defaultModel      = "deepseek/deepseek-chat-v3-0324"
	openRouterBaseURL = "https://openrouter.ai/api/v1"
)

// Client wraps openai-go pointed at OpenRouter.
type Client struct {
	inner openai.Client
	model string
}

// New creates a Client using OPENROUTER_API_KEY from env.
func New() (*Client, error) {
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY not set")
	}
	c := openai.NewClient(
		option.WithAPIKey(key),
		option.WithBaseURL(openRouterBaseURL),
	)
	return &Client{inner: c, model: defaultModel}, nil
}

// ToolDef defines a tool available to the model.
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

// Chat sends a message and returns the model's text response.
func (c *Client) Chat(ctx context.Context, req ChatRequest) (string, error) {
	params := c.buildParams(req)
	resp, err := c.inner.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("openrouter api: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty response")
	}
	return resp.Choices[0].Message.Content, nil
}

// ChatWithTools runs an agentic loop: calls the model, executes tools via
// executor, feeds results back — until finish_reason is "stop" or no tool calls.
func (c *Client) ChatWithTools(
	ctx context.Context,
	req ChatRequest,
	executor func(name string, input json.RawMessage) (string, error),
) (string, error) {
	params := c.buildParams(req)

	for {
		resp, err := c.inner.Chat.Completions.New(ctx, params)
		if err != nil {
			return "", fmt.Errorf("openrouter api: %w", err)
		}
		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("empty response")
		}

		choice := resp.Choices[0]

		if choice.FinishReason == "stop" || len(choice.Message.ToolCalls) == 0 {
			return choice.Message.Content, nil
		}

		// Append assistant message (with tool calls) back to history
		params.Messages = append(params.Messages, openai.ChatCompletionMessageParamUnion{
			OfAssistant: ptr(choice.Message.ToAssistantMessageParam()),
		})

		// Execute each tool and collect results
		for _, tc := range choice.Message.ToolCalls {
			result, err := executor(tc.Function.Name, json.RawMessage(tc.Function.Arguments))
			content := result
			if err != nil {
				content = fmt.Sprintf("error: %s", err)
			}
			params.Messages = append(params.Messages,
				openai.ToolMessage(content, tc.ID),
			)
		}
	}
}

// buildParams constructs ChatCompletionNewParams from a ChatRequest.
func (c *Client) buildParams(req ChatRequest) openai.ChatCompletionNewParams {
	var messages []openai.ChatCompletionMessageParamUnion
	if req.System != "" {
		messages = append(messages, openai.SystemMessage(req.System))
	}
	messages = append(messages, openai.UserMessage(req.User))

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(c.model),
		Messages: messages,
	}

	if len(req.Tools) > 0 {
		params.Tools = buildTools(req.Tools)
	}

	return params
}

func buildTools(defs []ToolDef) []openai.ChatCompletionToolParam {
	tools := make([]openai.ChatCompletionToolParam, len(defs))
	for i, t := range defs {
		tools[i] = openai.ChatCompletionToolParam{
			Type: "function",
			Function: shared.FunctionDefinitionParam{
				Name:        t.Name,
				Description: param.NewOpt(t.Description),
				Parameters:  toFunctionParameters(t.InputSchema),
			},
		}
	}
	return tools
}

func toFunctionParameters(v any) shared.FunctionParameters {
	if v == nil {
		return shared.FunctionParameters{"type": "object", "properties": map[string]any{}}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return shared.FunctionParameters{"type": "object", "properties": map[string]any{}}
	}
	var m shared.FunctionParameters
	_ = json.Unmarshal(b, &m)
	return m
}

func ptr[T any](v T) *T { return &v }
