package bot

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pathakanu/myMemo/internal/config"
	"github.com/pathakanu/myMemo/internal/model"
	myopenai "github.com/pathakanu/myMemo/internal/openai"
	"github.com/pathakanu/myMemo/internal/twilio"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// Bot coordinates reminder persistence, messaging, and scheduling.
type Bot struct {
	cfg    *config.Config
	db     *gorm.DB
	openAI *myopenai.Client
	twilio *twilio.Client
	cron   *cron.Cron
	state  *conversationStore
	logger *log.Logger
}

// New creates a fully configured Bot instance.
func New(cfg *config.Config, db *gorm.DB, openAI *myopenai.Client, twilioClient *twilio.Client, logger *log.Logger) *Bot {
	c := cron.New(cron.WithLocation(cfg.LocalTimezone))
	b := &Bot{
		cfg:    cfg,
		db:     db,
		openAI: openAI,
		twilio: twilioClient,
		cron:   c,
		state:  newConversationStore(),
		logger: logger,
	}
	return b
}

// StartScheduler registers cron jobs and starts the scheduler loop.
func (b *Bot) StartScheduler() error {
	_, err := b.cron.AddFunc("0 8 * * *", func() {
		go b.sendScheduledReminders()
	})
	if err != nil {
		return err
	}
	b.cron.Start()
	return nil
}

// StopScheduler stops the cron scheduler gracefully.
func (b *Bot) StopScheduler() {
	ctx := b.cron.Stop()
	<-ctx.Done()
}

// Handler returns the HTTP handler for incoming Twilio messages.
func (b *Bot) Handler() http.HandlerFunc {
	return b.handleIncomingMessage
}

// handleIncomingMessage processes Twilio webhook POST requests.
func (b *Bot) handleIncomingMessage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		b.logger.Printf("webhook: parse error: %v", err)
		b.writeTwilioResponse(w, "Sorry, I couldn't understand that request.")
		return
	}

	from := r.FormValue("From")
	body := strings.TrimSpace(r.FormValue("Body"))
	if from == "" || body == "" {
		b.writeTwilioResponse(w, "I need a message to work with. Please try again.")
		return
	}

	userID := sanitizeWhatsAppNumber(from)
	lowerBody := strings.ToLower(body)

	if b.state.IsAwaitingPriority(userID) {
		b.handlePriorityResponse(w, userID, body)
		return
	}

	intent, keyword := b.determineIntent(r.Context(), body, lowerBody)

	switch intent {
	case myopenai.IntentListReminders:
		list := b.listReminders(userID)
		if list == "" {
			b.writeTwilioResponse(w, "You have no reminders yet. Send me one to get started!")
			return
		}
		b.writeTwilioResponse(w, list)
	case myopenai.IntentClearReminders:
		if err := b.deleteReminder(userID, ""); err != nil {
			b.logger.Printf("clear reminders: %v", err)
			b.writeTwilioResponse(w, "Hmm, I couldn't clear your reminders. Please try again later.")
			return
		}
		b.writeTwilioResponse(w, "All reminders cleared.")
	case myopenai.IntentDeleteReminder:
		if keyword == "" {
			b.writeTwilioResponse(w, "Tell me which reminder to delete, e.g. 'delete reminder about milk'.")
			return
		}
		if err := b.deleteReminder(userID, keyword); err != nil {
			b.logger.Printf("delete reminder: %v", err)
			b.writeTwilioResponse(w, "I couldn't find that reminder.")
			return
		}
		b.writeTwilioResponse(w, fmt.Sprintf("Deleted reminders matching '%s'.", keyword))
	case myopenai.IntentHelp:
		b.writeTwilioResponse(w, helpResponse())
	default:
		b.state.SetPendingMessage(userID, body)
		b.writeTwilioResponse(w, b.askForPriority())
	}
}

func (b *Bot) determineIntent(ctx context.Context, message, lowerMessage string) (myopenai.Intent, string) {
	if isClearAllRequest(lowerMessage) {
		return myopenai.IntentClearReminders, ""
	}
	if isListRequest(lowerMessage) {
		return myopenai.IntentListReminders, ""
	}
	if keyword := extractDeleteKeyword(message); keyword != "" {
		return myopenai.IntentDeleteReminder, keyword
	}

	if b.openAI == nil {
		return myopenai.IntentAddReminder, ""
	}

	intent, err := b.openAI.ClassifyIntent(ctx, message)
	if err != nil {
		if !errors.Is(err, myopenai.ErrClientNotInitialised) {
			b.logger.Printf("intent classification error: %v", err)
		}
		return myopenai.IntentAddReminder, ""
	}

	switch intent {
	case myopenai.IntentDeleteReminder:
		return intent, extractDeleteKeyword(message)
	case myopenai.IntentListReminders,
		myopenai.IntentClearReminders,
		myopenai.IntentHelp,
		myopenai.IntentAddReminder:
		return intent, ""
	default:
		return myopenai.IntentAddReminder, ""
	}
}

func (b *Bot) handlePriorityResponse(w http.ResponseWriter, userID, priorityText string) {
	priority, err := strconv.Atoi(strings.TrimSpace(priorityText))
	if err != nil || priority < 1 || priority > 5 {
		b.writeTwilioResponse(w, "Please send a priority between 1 (lowest) and 5 (highest).")
		return
	}

	content, ok := b.state.PopPendingMessage(userID)
	if !ok {
		b.writeTwilioResponse(w, "I lost track of that reminder. Please send it again.")
		return
	}

	summary := b.summarizeReminderWithOpenAI(content)
	if err := b.saveReminder(userID, content, priority, summary); err != nil {
		b.logger.Printf("save reminder: %v", err)
		b.writeTwilioResponse(w, "I couldn't save the reminder. Please try again.")
		return
	}

	b.writeTwilioResponse(w, fmt.Sprintf("Got it! I'll remind you: %s (priority %d).", summary, priority))
}

// askForPriority prompts the user to provide a priority for their reminder.
func (b *Bot) askForPriority() string {
	return "What priority should I set? Reply with a number between 1 (low) and 5 (high)."
}

// saveReminder persists a reminder to the database.
func (b *Bot) saveReminder(userID, message string, priority int, summary string) error {
	reminder := &model.Reminder{
		UserID:   userID,
		Content:  message,
		Priority: priority,
		Summary:  summary,
	}
	return b.db.Create(reminder).Error
}

// listReminders returns a human-readable list of reminders for a user.
func (b *Bot) listReminders(userID string) string {
	var reminders []model.Reminder
	if err := b.db.Where("user_id = ?", userID).
		Order("priority DESC, created_at ASC").
		Find(&reminders).Error; err != nil {
		b.logger.Printf("list reminders error: %v", err)
		return ""
	}
	if len(reminders) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Here are your reminders:\n")
	for i, r := range reminders {
		sb.WriteString(fmt.Sprintf("%d. [%d] %s — saved %s\n", i+1, r.Priority, fallback(r.Summary, r.Content), r.CreatedAt.Format("Jan 02 15:04")))
	}
	return sb.String()
}

// deleteReminder deletes a reminder based on a keyword. When keyword is empty all reminders are removed.
func (b *Bot) deleteReminder(userID, keyword string) error {
	query := b.db.Where("user_id = ?", userID)
	if strings.TrimSpace(keyword) != "" {
		query = query.Where("LOWER(content) LIKE ?", "%"+strings.ToLower(keyword)+"%")
	}
	return query.Delete(&model.Reminder{}).Error
}

// sendScheduledReminders sends all reminders sorted by priority starting at 8AM local time.
func (b *Bot) sendScheduledReminders() {
	var users []string
	if err := b.db.Model(&model.Reminder{}).Distinct().Pluck("user_id", &users).Error; err != nil {
		b.logger.Printf("scheduler: fetch users: %v", err)
		return
	}

	for _, userID := range users {
		go b.dispatchUserReminders(userID)
	}
}

func (b *Bot) dispatchUserReminders(userID string) {
	var reminders []model.Reminder
	if err := b.db.Where("user_id = ?", userID).
		Order("priority DESC, created_at ASC").
		Find(&reminders).Error; err != nil {
		b.logger.Printf("scheduler: user %s: %v", userID, err)
		return
	}
	if len(reminders) == 0 {
		return
	}

	for index, reminder := range reminders {
		delay := time.Duration(index) * time.Hour
		time.AfterFunc(delay, func(rem model.Reminder) func() {
			return func() {
				message := fmt.Sprintf("Reminder: %s (priority %d)", fallback(rem.Summary, rem.Content), rem.Priority)
				if err := b.twilio.SendWhatsAppMessage(userID, message); err != nil {
					b.logger.Printf("scheduler: send reminder: %v", err)
				}
			}
		}(reminder))
	}
}

// summarizeReminderWithOpenAI generates a short summary for the reminder content.
func (b *Bot) summarizeReminderWithOpenAI(content string) string {
	ctx := context.Background()
	summary, err := b.openAI.SummarizeReminder(ctx, content)
	if err != nil {
		b.logger.Printf("openai summarise error: %v", err)
		return content
	}
	return summary
}

func (b *Bot) writeTwilioResponse(w http.ResponseWriter, message string) {
	twiml := struct {
		XMLName xml.Name `xml:"Response"`
		Message string   `xml:"Message"`
	}{
		Message: message,
	}

	w.Header().Set("Content-Type", "application/xml")
	if err := xml.NewEncoder(w).Encode(twiml); err != nil {
		b.logger.Printf("twilio response encode: %v", err)
	}
}

func isListRequest(body string) bool {
	return strings.Contains(body, "show my reminders") ||
		strings.Contains(body, "list my reminders") ||
		strings.Contains(body, "show reminders") ||
		strings.Contains(body, "list reminders") ||
		(strings.Contains(body, "list") && strings.Contains(body, "reminder"))
}

func isClearAllRequest(body string) bool {
	return body == "clear all reminders" ||
		body == "clear reminders" ||
		body == "delete all reminders"
}

func sanitizeWhatsAppNumber(from string) string {
	// Twilio prepends whatsapp: to the number.
	return strings.TrimPrefix(from, "whatsapp:")
}

func fallback(primary, secondary string) string {
	if strings.TrimSpace(primary) == "" {
		return secondary
	}
	return primary
}

func helpResponse() string {
	return "You can say things like:\n- \"Remind me to pay rent\" to add a reminder\n- \"List reminders\" to see everything saved\n- \"Delete reminder about rent\" to remove one\n- \"Clear all reminders\" to wipe everything"
}

var deleteKeywordRegex = regexp.MustCompile(`(?i)delete(?:\s+reminder(?:s)?(?:\s+about)?)?\s*(.*)`)

func extractDeleteKeyword(message string) string {
	matches := deleteKeywordRegex.FindStringSubmatch(message)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

type conversationStore struct {
	mu    sync.RWMutex
	state map[string]conversationState
}

type conversationState struct {
	AwaitingPriority bool
	PendingMessage   string
}

func newConversationStore() *conversationStore {
	return &conversationStore{
		state: make(map[string]conversationState),
	}
}

func (c *conversationStore) SetPendingMessage(userID, message string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state[userID] = conversationState{
		AwaitingPriority: true,
		PendingMessage:   message,
	}
}

func (c *conversationStore) PopPendingMessage(userID string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	state, ok := c.state[userID]
	if !ok {
		return "", false
	}
	delete(c.state, userID)
	return state.PendingMessage, true
}

func (c *conversationStore) IsAwaitingPriority(userID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	state, ok := c.state[userID]
	return ok && state.AwaitingPriority
}

// DecodeTwilioForm extracts the POST form data into a map for convenience.
func DecodeTwilioForm(values url.Values) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		if len(value) > 0 {
			result[key] = value[0]
		}
	}
	return result
}
