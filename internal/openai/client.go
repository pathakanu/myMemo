package openai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// Client wraps the OpenAI SDK and provides utility helpers.
type Client struct {
	apiKey string
	client *openai.Client
	model  openai.ChatModel
}

// ErrClientNotInitialised is returned when attempting to call the API without a configured client.
var ErrClientNotInitialised = errors.New("openai client not initialised")

// Intent represents the high-level action inferred from a user message.
type Intent string

const (
	// IntentUnknown indicates the message intent could not be resolved.
	IntentUnknown Intent = "unknown"
	// IntentAddReminder instructs the bot to capture a new reminder.
	IntentAddReminder Intent = "add_reminder"
	// IntentListReminders asks the bot to list current reminders.
	IntentListReminders Intent = "list_reminders"
	// IntentDeleteReminder requests deletion of a specific reminder.
	IntentDeleteReminder Intent = "delete_reminder"
	// IntentClearReminders requests that all reminders be removed.
	IntentClearReminders Intent = "clear_reminders"
	// IntentHelp asks for usage guidance.
	IntentHelp Intent = "help"
)

// New returns an OpenAI client when apiKey is provided, otherwise nil is returned.
func New(apiKey string) *Client {
	if apiKey == "" {
		return &Client{}
	}
	client := openai.NewClient(option.WithAPIKey(apiKey))
	return &Client{
		apiKey: apiKey,
		client: &client,
		model:  openai.ChatModelGPT4oMini,
	}
}

// SummarizeReminder asks the model to summarise the provided content.
func (c *Client) SummarizeReminder(ctx context.Context, content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("content cannot be empty")
	}
	if c.client == nil {
		// fallback: return truncated content when API key is missing.
		if len(content) > 80 {
			return content[:80] + "...", nil
		}
		return content, nil
	}

	req := openai.ChatCompletionNewParams{
		Model: c.model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			{
				OfSystem: &openai.ChatCompletionSystemMessageParam{
					Content: openai.ChatCompletionSystemMessageParamContentUnion{
						OfString: openai.String("You summarise reminder texts in one short sentence."),
					},
				},
			},
			{
				OfUser: &openai.ChatCompletionUserMessageParam{
					Content: openai.ChatCompletionUserMessageParamContentUnion{
						OfString: openai.String(fmt.Sprintf("Summarise the following reminder in one sentence: %s", content)),
					},
				},
			},
		},
		Temperature:         openai.Float(0.3),
		MaxCompletionTokens: openai.Int(60),
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	resp, err := c.client.Chat.Completions.New(ctx, req)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no completion received")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

// ClassifyIntent uses the language model to infer the user's intent.
func (c *Client) ClassifyIntent(ctx context.Context, content string) (Intent, error) {
	if strings.TrimSpace(content) == "" {
		return IntentUnknown, fmt.Errorf("content cannot be empty")
	}
	if c.client == nil {
		return IntentUnknown, ErrClientNotInitialised
	}

	req := openai.ChatCompletionNewParams{
		Model: c.model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			{
				OfSystem: &openai.ChatCompletionSystemMessageParam{
					Content: openai.ChatCompletionSystemMessageParamContentUnion{
						OfString: openai.String("Classify the user's request for a reminder bot. Reply with exactly one label: add_reminder, list_reminders, delete_reminder, clear_reminders, help, or unknown."),
					},
				},
			},
			{
				OfUser: &openai.ChatCompletionUserMessageParam{
					Content: openai.ChatCompletionUserMessageParamContentUnion{
						OfString: openai.String(content),
					},
				},
			},
		},
		Temperature:         openai.Float(0.0),
		MaxCompletionTokens: openai.Int(8),
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := c.client.Chat.Completions.New(ctx, req)
	if err != nil {
		return IntentUnknown, err
	}
	if len(resp.Choices) == 0 {
		return IntentUnknown, fmt.Errorf("no completion received")
	}

	label := strings.TrimSpace(resp.Choices[0].Message.Content)
	switch Intent(strings.ToLower(label)) {
	case IntentAddReminder:
		return IntentAddReminder, nil
	case IntentListReminders:
		return IntentListReminders, nil
	case IntentDeleteReminder:
		return IntentDeleteReminder, nil
	case IntentClearReminders:
		return IntentClearReminders, nil
	case IntentHelp:
		return IntentHelp, nil
	default:
		return IntentUnknown, nil
	}
}
