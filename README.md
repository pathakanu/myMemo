# myMemo WhatsApp Reminder Bot

myMemo is a Go-based WhatsApp bot that lets users add, list, summarise, and delete reminders with daily, priority-ordered notifications. It uses Twilio’s WhatsApp API for messaging, OpenAI for natural-language summaries, and SQLite/PostgreSQL for storage.

## Features
- Two-step reminder capture with priority prompts (1–5).
- Automatic one-line summaries using OpenAI GPT models.
- Daily reminder dispatch at 8AM in the configured timezone, spaced hourly by priority.
- Commands for listing, deleting by keyword, and clearing reminders.
- Pluggable SQLite (default) or PostgreSQL persistence via GORM.

## Prerequisites
- Go 1.22+ (module targets Go 1.23).
- A Twilio account with WhatsApp sandbox or approved sender.
- An OpenAI API key (optional but recommended; bot falls back to raw text when absent).
- SQLite (bundled) or PostgreSQL if you prefer a server database.

## Local Setup
1. **Clone and enter the project directory**
   ```bash
   git clone https://github.com/pathakanu/myMemo.git
   cd myMemo
   ```

2. **Copy and populate environment variables**
   ```bash
   cp .env.example .env
   ```
   Required values:
   - `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`: from the Twilio console.
   - `TWILIO_WHATSAPP_NUMBER`: WhatsApp-enabled Twilio number (e.g. `+1415...`).
   - `OPENAI_API_KEY`: OpenAI secret key (`sk-...`). Leave blank to disable summaries.
   - `DATABASE_URL`: Optional PostgreSQL connection string. Leave empty to use local `reminders.db` (SQLite).
   - `LOCAL_TIMEZONE`: IANA timezone (e.g. `America/New_York`). Defaults to the host locale.

3. **Install Go dependencies**
   ```bash
   go mod tidy
   ```

4. **Run the server**
   ```bash
   go run ./...
   ```
   The HTTP server listens on the port defined by `PORT` (defaults to `8080`).

## Twilio Webhook Configuration
1. Expose your local server (e.g. with `ngrok http 8080`).
2. In the Twilio Console, set the WhatsApp sandbox (or phone number) incoming message webhook to:
   ```
   https://<your-public-host>/twilio/webhook
   ```
3. Subscribe your personal WhatsApp number to the sandbox (Twilio provides the join code). Messages you send to the sandbox will now hit the bot.

## Scheduler Behaviour
- At 08:00 (configured timezone) the bot fetches each user’s reminders ordered by priority (5 → 1).
- Reminders send via WhatsApp using Twilio, with each subsequent reminder spaced one hour after the previous.
- You can adjust the cron expression in `internal/bot/bot.go` if you need different timing.

## Database Notes
- With the default SQLite configuration, data is stored in `reminders.db` in the project root.
- To use PostgreSQL, populate `DATABASE_URL` with a standard connection string, e.g. `postgres://user:pass@host:5432/database`.
- GORM automatically applies the `Reminder` schema on startup.

## Development Tips
- Modify `.env` values and restart the server to refresh configuration.
- The OpenAI summariser times out after 15 seconds; errors fall back to the original reminder text.
- Logging is emitted with a `[myMemo]` prefix; use it to inspect scheduler activity and webhook handling.

## Testing the Flow
1. Send a WhatsApp message like “Remind me to buy milk”.
2. Bot replies asking for priority.
3. Reply with a number between 1 and 5.
4. Bot confirms with the saved summary.
5. Send “show my reminders” to view all entries.
6. Use “delete milk” or “clear all reminders” as needed.

## Next Steps
- Containerise the service for deployment.
- Add authentication/ACLs if multi-user access beyond WhatsApp IDs is needed.
- Extend with voice prompts, recurring schedules, or analytics dashboards.
