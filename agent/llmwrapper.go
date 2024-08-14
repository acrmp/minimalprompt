// Package agent implements a wrapper around a LLM with tools for command
// and filesystem access.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/tmc/langchaingo/llms"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . CommandExecutor
type CommandExecutor interface {
	Execute(command string) (string, error)
}

//counterfeiter:generate . FileWriter
type FileWriter interface {
	WriteFile(path, content string) error
}

//counterfeiter:generate . Model
type Model interface {
	GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error)
}

//counterfeiter:generate . Prompter
type Prompter interface {
	Prompt(input string) (string, error)
}

// A LLMWrapper implements a wrapper around a LLM.
type LLMWrapper struct {
	logger          *slog.Logger
	persona         string
	prompt          string
	model           Model
	commandExecutor CommandExecutor
	fileWriter      FileWriter
	prompter        Prompter
	history         []llms.MessageContent
}

// NewLLMWrapper creates a LLMWrapper.
// The persona is set as the LLM system prompt and the prompt is the initial prompt.
func NewLLMWrapper(logger *slog.Logger, persona string, prompt string, m Model, ce CommandExecutor, fw FileWriter, p Prompter) *LLMWrapper {
	return &LLMWrapper{logger: logger, persona: persona, prompt: prompt, model: m, commandExecutor: ce, fileWriter: fw, prompter: p}
}

// Run executes against the LLM.
func (l *LLMWrapper) Run(ctx context.Context) error {
	l.history = []llms.MessageContent{
		{
			Role: llms.ChatMessageTypeSystem,
			Parts: []llms.ContentPart{
				llms.TextPart(l.persona),
			},
		},
		{
			Role: llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{
				llms.TextPart(l.prompt),
			},
		},
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			r, err := l.model.GenerateContent(
				ctx,
				l.history,
				llms.WithTools([]llms.Tool{executeCommandTool, writeFileTool}),
			)
			if err != nil {
				return err
			}
			if err = l.processResponse(r); err != nil {
				return err
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func (l *LLMWrapper) processResponse(r *llms.ContentResponse) error {
	if len(r.Choices) == 0 {
		return nil
	}
	for _, c := range r.Choices {
		if len(c.Content) > 0 {
			l.logger.Info("AI says", "content", c.Content)
		}
		if err := l.performToolCalls(c.ToolCalls); err != nil {
			return err
		}
		if len(c.ToolCalls) > 0 {
			break
		}
		if c.StopReason == "end_turn" {
			if err := l.promptUser(c.Content); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *LLMWrapper) promptUser(q string) error {
	l.recordText(llms.ChatMessageTypeAI, q)

	prompt, err := l.prompter.Prompt(q)
	if err != nil {
		return err
	}

	l.recordText(llms.ChatMessageTypeHuman, prompt)
	return nil
}

func (l *LLMWrapper) recordText(role llms.ChatMessageType, text string) {
	r := llms.MessageContent{
		Role: role,
		Parts: []llms.ContentPart{
			llms.TextPart(text),
		},
	}
	l.history = append(l.history, r)
}

func (l *LLMWrapper) recordToolCall(tc llms.ToolCall) {
	r := llms.MessageContent{
		Role: llms.ChatMessageTypeAI,
		Parts: []llms.ContentPart{
			llms.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				FunctionCall: &llms.FunctionCall{
					Name:      tc.FunctionCall.Name,
					Arguments: tc.FunctionCall.Arguments,
				},
			},
		},
	}
	l.history = append(l.history, r)
}

func (l *LLMWrapper) recordToolResponse(tc llms.ToolCall, content string) {
	l.history = append(l.history, llms.MessageContent{
		Role: llms.ChatMessageTypeTool,
		Parts: []llms.ContentPart{
			llms.ToolCallResponse{
				ToolCallID: tc.ID,
				Name:       tc.FunctionCall.Name,
				Content:    content,
			},
		},
	})
}

func (l *LLMWrapper) performToolCalls(calls []llms.ToolCall) error {
	for _, tc := range calls {
		l.recordToolCall(tc)

		switch tc.FunctionCall.Name {
		case "executeCommand":
			var args struct {
				Command string
			}
			err := json.Unmarshal([]byte(tc.FunctionCall.Arguments), &args)
			if err != nil {
				return fmt.Errorf("could not parse tool call arguments: %q: %w", tc.FunctionCall.Name, err)
			}
			prefix := "The command ran successfully with the output"
			output, err := l.commandExecutor.Execute(args.Command)
			if err != nil {
				prefix = "The command failed with the output"
			}
			l.recordToolResponse(tc, fmt.Sprintf("%s:\n%s", prefix, output))
		case "writeFile":
			var args struct {
				Path    string
				Content string
			}
			err := json.Unmarshal([]byte(tc.FunctionCall.Arguments), &args)
			if err != nil {
				return fmt.Errorf("could not parse tool call arguments: %q: %w", tc.FunctionCall.Name, err)
			}

			if err := l.fileWriter.WriteFile(args.Path, args.Content); err != nil {
				return fmt.Errorf("tool call failed: %q: %w", tc.FunctionCall.Name, err)
			}
			l.recordToolResponse(tc, "ok")
		default:
			return fmt.Errorf("unrecognised tool call from model: %q", tc.FunctionCall.Name)
		}
	}
	return nil
}

var executeCommandTool = llms.Tool{
	Type: "function",
	Function: &llms.FunctionDefinition{
		Name:        "executeCommand",
		Description: "Execute an operating system bash command",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "The command to execute",
				},
			},
			"required": []string{"command"},
		},
	},
}
var writeFileTool = llms.Tool{
	Type: "function",
	Function: &llms.FunctionDefinition{
		Name:        "writeFile",
		Description: "Write a file to the filesystem",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"content": map[string]any{
					"type":        "string",
					"description": "The content of the file as a string",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "The relative path of the file within the project",
				},
			},
			"required": []string{"content", "path"},
		},
	},
}
