# myMemo Test Scenarios

These scenarios focus on the conversational logic served through the Twilio webhook while using an in-memory SQLite database and mocked external services.

1. **Add Reminder Success**
   - Incoming message: “Remind me to buy milk tomorrow.”
   - Bot prompts for priority.
   - User replies “3”.
   - Bot replies with confirmation containing summary and priority.

2. **Invalid Priority Handling**
   - After the priority prompt, replies “ten”.
   - Bot asks again for a numeric priority between 1 and 5.
   - Reply “6”; bot still rejects.
   - Reply “2”; bot confirms and saves reminder once valid.

3. **List Reminders**
   - With at least one saved reminder, message “list reminders”.
   - Bot responds with ordered list including summary, priority, and timestamp text.

4. **Delete by Keyword**
   - After adding reminders with distinct keywords, send “delete reminder about rent”.
   - Bot acknowledges deletion of matching reminders.
   - Listing afterwards omits the deleted items.

5. **Delete by Indices**
   - With multiple reminders saved, send “delete 1,3”.
   - Bot confirms indices deleted.
   - Listing shows remaining reminders only.

6. **Help Intent**
   - Message “help”.
   - Bot returns usage guidance describing supported commands.
