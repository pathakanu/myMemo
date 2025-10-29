package twilio

import (
	"fmt"

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
	params := &openapi.CreateMessageParams{}
	params.SetTo(fmt.Sprintf("whatsapp:%s", to))
	params.SetFrom(fmt.Sprintf("whatsapp:%s", c.fromWhatsApp))
	params.SetBody(body)

	_, err := c.client.Api.CreateMessage(params)
	return err
}
