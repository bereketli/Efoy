-- +goose Up
-- Notifications are rendered in the recipient's language when queued, so the
-- inbox and every retry show the same text.
ALTER TABLE notifications
  ADD COLUMN title      text,
  ADD COLUMN body       text,
  ADD COLUMN last_error text;
CREATE INDEX ix_notifications_queued ON notifications (created_at) WHERE status = 'QUEUED';

-- Templates (FR-NOT-2): Go text/template, one row per code, channel and
-- language. Afaan Oromo arrives with the translations on day 25; missing
-- languages fall back to English.
INSERT INTO notification_templates (code, channel, language, title, body)
SELECT m.code, c.channel::notif_channel, m.language::app_language, m.title, m.body
  FROM (VALUES
    ('notification.test', 'en', 'Efoy test notification', 'This is a test message from Efoy.'),
    ('notification.test', 'am', 'የእፋይ የሙከራ መልዕክት', 'ይህ ከእፋይ የተላከ የሙከራ መልዕክት ነው።'),
    ('document.approved', 'en', 'Document approved', 'Your {{.DocType}} was approved.'),
    ('document.approved', 'am', 'ሰነድ ጸድቋል', '{{.DocType}} ሰነድዎ ጸድቋል።'),
    ('document.rejected', 'en', 'Document rejected', 'Your {{.DocType}} was rejected: {{.Reason}} Please upload a new one.'),
    ('document.rejected', 'am', 'ሰነድ ውድቅ ተደርጓል', '{{.DocType}} ሰነድዎ ውድቅ ተደርጓል፦ {{.Reason}} እባክዎ አዲስ ይላኩ።'),
    ('document.expiring', 'en', 'Document expiring soon', 'Your {{.DocType}} expires on {{.ExpiresOn}} ({{.Days}} days left). Upload the renewed document to keep driving.'),
    ('document.expiring', 'am', 'የሰነድ ጊዜ ሊያልፍ ነው', '{{.DocType}} ሰነድዎ በ{{.ExpiresOn}} ጊዜው ያልፋል ({{.Days}} ቀናት ቀርተዋል)። ማሽከርከርዎን ለመቀጠል የታደሰውን ሰነድ ይላኩ።'),
    ('document.expired', 'en', 'Document expired', 'Your {{.DocType}} has expired. Upload the renewed document.'),
    ('document.expired', 'am', 'የሰነድ ጊዜ አልፏል', 'የ{{.DocType}} ሰነድዎ ጊዜ አልፏል። የታደሰውን ሰነድ ይላኩ።'),
    ('driver.activated', 'en', 'You are active on Efoy', 'Your documents are approved. You can now be assigned routes and standby duty.'),
    ('driver.activated', 'am', 'በእፋይ ላይ ንቁ ሆነዋል', 'ሰነዶችዎ ጸድቀዋል። አሁን መስመሮችና የተጠባባቂ ስራ ሊመደቡልዎ ይችላሉ።'),
    ('driver.suspended', 'en', 'Driver account suspended', 'Your Efoy driver account is suspended: {{.Reason}}'),
    ('driver.suspended', 'am', 'የአሽከርካሪ መለያዎ ታግዷል', 'የእፋይ አሽከርካሪ መለያዎ ታግዷል፦ {{.Reason}}'),
    ('guardian.added', 'en', 'You were added as a guardian', '{{.InviterName}} added you as a guardian of {{.RiderName}} on Efoy. Install the Efoy app to follow their trips.'),
    ('guardian.added', 'am', 'አሳዳጊ ሆነው ተጨምረዋል', '{{.InviterName}} በእፋይ ላይ የ{{.RiderName}} አሳዳጊ አድርገው ጨምረውዎታል። ጉዞዎቹን ለመከታተል የእፋይ መተግበሪያን ይጫኑ።')
  ) AS m(code, language, title, body)
 CROSS JOIN (VALUES ('PUSH'), ('SMS'), ('IN_APP')) AS c(channel)
ON CONFLICT (code, channel, language) DO UPDATE SET title = EXCLUDED.title, body = EXCLUDED.body;

-- +goose Down
DELETE FROM notification_templates
 WHERE code IN ('notification.test','document.approved','document.rejected','document.expiring',
                'document.expired','driver.activated','driver.suspended','guardian.added');
DROP INDEX ix_notifications_queued;
ALTER TABLE notifications DROP COLUMN last_error, DROP COLUMN body, DROP COLUMN title;
