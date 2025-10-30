package twilio

import (
	"fmt"
	"strings"

	// "github.com/caarlos0/env/v11"
	twilio "github.com/twilio/twilio-go"
	openapi "github.com/twilio/twilio-go/rest/api/v2010"
)

// Client wraps Twilio messaging operations required by the bot.
type Client struct {
	client       *twilio.RestClient
	fromWhatsApp string
}

// New creates a Twilio client bound to the configured WhatsApp sender number.
func New(accountSID, authToken, fromWhatsApp string) *Client {
	return &Client{
		client:       twilio.NewRestClientWithParams(twilio.ClientParams{Username: accountSID, Password: authToken}),
		fromWhatsApp: fromWhatsApp,
	}
}

// SendWhatsAppMessage sends a WhatsApp message via Twilio's API.
func (c *Client) SendWhatsAppMessage(to, body string) error {
	if c.client == nil {
		return fmt.Errorf("twilio client not initialised")
	}

	sender := normalizeWhatsAppAddress(c.fromWhatsApp)
	if sender == "" {
		return fmt.Errorf("twilio sender WhatsApp number is not configured")
	}

	recipient := normalizeWhatsAppAddress(to)
	if recipient == "" {
		return fmt.Errorf("recipient number missing or invalid")
	}

	fmt.Printf("Sending WhatsApp message to %s via %s: %s\n", recipient, sender, body)

	params := &openapi.CreateMessageParams{}
	params.SetTo(recipient)
	params.SetFrom(sender)
	params.SetBody(body)

	resp, err := c.client.Api.CreateMessage(params)
	if err != nil {
		return fmt.Errorf("twilio send message error: %w", err)
	}

	fmt.Printf("Twilio message sent, SID: %s\n", *resp.Sid)
	return err
}

func normalizeWhatsAppAddress(number string) string {
	trimmed := strings.TrimSpace(number)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "whatsapp:") {
		return trimmed
	}
	if strings.HasPrefix(trimmed, "+") {
		return "whatsapp:" + trimmed
	}
	return "whatsapp:+" + trimmed
}
