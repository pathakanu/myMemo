package bot

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/pathakanu/myMemo/internal/config"
	"github.com/pathakanu/myMemo/internal/model"
	myopenai "github.com/pathakanu/myMemo/internal/openai"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestBot(t *testing.T) *Bot {
	t.Helper()

	name := strings.ReplaceAll(t.Name(), "/", "_")
	dsn := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared&_fk=1", name, time.Now().UnixNano())

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite memory: %v", err)
	}
	if err := db.AutoMigrate(&model.Reminder{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	return &Bot{
		cfg:    &config.Config{LocalTimezone: time.UTC},
		db:     db,
		openAI: myopenai.New(""),
		twilio: nil,
		cron:   nil,
		state:  newConversationStore(),
		logger: log.New(io.Discard, "", 0),
	}
}

func TestParseIndices(t *testing.T) {
	t.Parallel()

	cases := map[string][]int{
		"1 2 3":      {1, 2, 3},
		"1,2,3":      {1, 2, 3},
		" 3 , 2 , 1": {3, 2, 1},
		"5":          {5},
		"":           nil,
		"0,1":        nil,
		"-1":         nil,
		"1,a":        nil,
	}

	for input, want := range cases {
		got := parseIndices(input)
		if len(got) != len(want) {
			t.Fatalf("parseIndices(%q) = %v, want %v", input, got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("parseIndices(%q)[%d] = %d, want %d", input, i, got[i], want[i])
			}
		}
	}
}

func TestDeleteReminderByIndices(t *testing.T) {
	t.Parallel()
	b := newTestBot(t)

	seedReminders(t, b, []model.Reminder{
		{UserID: "user", Content: "alpha", Priority: 5},
		{UserID: "user", Content: "beta", Priority: 3},
		{UserID: "user", Content: "gamma", Priority: 1},
	})

	msg, err := b.deleteReminder("user", "1,3")
	if err != nil {
		t.Fatalf("deleteReminder returned error: %v", err)
	}
	if want := "Deleted reminder(s): 1, 3."; msg != want {
		t.Fatalf("unexpected message: got %q want %q", msg, want)
	}

	var reminders []model.Reminder
	if err := b.db.Where("user_id = ?", "user").Order("created_at asc").Find(&reminders).Error; err != nil {
		t.Fatalf("fetch reminders: %v", err)
	}
	if len(reminders) != 1 || reminders[0].Content != "beta" {
		t.Fatalf("expected only \"beta\" reminder remaining, got %+v", reminders)
	}

	if _, err := b.deleteReminder("user", "4"); err == nil {
		t.Fatalf("expected error for invalid index, got nil")
	}
}

func TestDeleteReminderByKeyword(t *testing.T) {
	t.Parallel()
	b := newTestBot(t)

	seedReminders(t, b, []model.Reminder{
		{UserID: "user", Content: "pay rent", Priority: 4},
		{UserID: "user", Content: "buy milk", Priority: 2},
	})

	msg, err := b.deleteReminder("user", "rent")
	if err != nil {
		t.Fatalf("deleteReminder by keyword error: %v", err)
	}
	if want := "Deleted reminders matching 'rent'."; msg != want {
		t.Fatalf("unexpected message: got %q want %q", msg, want)
	}

	var count int64
	if err := b.db.Model(&model.Reminder{}).Where("user_id = ?", "user").Count(&count).Error; err != nil {
		t.Fatalf("count reminders: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one reminder remaining, got %d", count)
	}

	_, err = b.deleteReminder("user", "doctor")
	if err == nil {
		t.Fatalf("expected error when keyword not found")
	}
}

func TestListRemindersFormatting(t *testing.T) {
	t.Parallel()
	b := newTestBot(t)

	seedReminders(t, b, []model.Reminder{
		{UserID: "user", Content: "task one", Summary: "Task one", Priority: 5},
		{UserID: "user", Content: "task two", Summary: "Task two", Priority: 2},
	})

	output := b.listReminders("user")
	if output == "" {
		t.Fatalf("expected non-empty list output")
	}
	if !containsAll(output, []string{"Here are your reminders", "Task one", "Task two", "[5]"}) {
		t.Fatalf("unexpected list output: %q", output)
	}
}

func TestSummarizeReminderFallback(t *testing.T) {
	t.Parallel()
	client := myopenai.New("")
	ctx := context.Background()
	content := "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed varius aliquet felis."

	summary, err := client.SummarizeReminder(ctx, content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summary) == 0 {
		t.Fatalf("expected fallback summary, got empty string")
	}
}

// seedReminders inserts reminders and updates CreatedAt to ensure ordering.
func seedReminders(t *testing.T, b *Bot, reminders []model.Reminder) {
	t.Helper()
	for i := range reminders {
		if reminders[i].CreatedAt.IsZero() {
			reminders[i].CreatedAt = time.Now().Add(time.Duration(i) * time.Minute)
		}
		if err := b.db.Create(&reminders[i]).Error; err != nil {
			t.Fatalf("seed reminder %d: %v", i, err)
		}
	}
}

func containsAll(haystack string, needles []string) bool {
	for _, needle := range needles {
		if needle != "" && !strings.Contains(haystack, needle) {
			return false
		}
	}
	return true
}
