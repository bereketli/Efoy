-- Day 9–11: payments and ledger, institution billing and renewals, daily trips.

-- +goose Up
-- Payment intents: what is being paid for, manual bank-transfer review, and
-- the last time the provider was asked for the authoritative status.
ALTER TABLE payment_intents
  ADD COLUMN purpose          text NOT NULL DEFAULT 'INITIAL' CHECK (purpose IN ('INITIAL', 'RENEWAL', 'INVOICE')),
  ADD COLUMN renewal_period   daterange,
  ADD COLUMN renewal_quote    jsonb,      -- price, shares and snapshot of the period being paid
  ADD COLUMN payer_phone      text,
  ADD COLUMN receipt_key      text,
  ADD COLUMN bank_reference   text,
  ADD COLUMN reviewed_by      uuid REFERENCES users(id),
  ADD COLUMN reviewed_at      timestamptz,
  ADD COLUMN last_verified_at timestamptz,
  ADD COLUMN updated_at       timestamptz NOT NULL DEFAULT now();
CREATE TRIGGER trg_payment_intents_updated BEFORE UPDATE ON payment_intents FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX ix_payment_intents_subscription ON payment_intents (subscription_id);
CREATE INDEX ix_payment_intents_invoice ON payment_intents (invoice_id);
CREATE INDEX ix_payment_intents_created ON payment_intents (created_at DESC, id DESC);
-- At most one open payment per subscription or invoice at a time.
CREATE UNIQUE INDEX uq_payment_intents_open_subscription ON payment_intents (subscription_id, purpose)
  WHERE status IN ('CREATED', 'PENDING') AND subscription_id IS NOT NULL;
CREATE UNIQUE INDEX uq_payment_intents_open_invoice ON payment_intents (invoice_id)
  WHERE status IN ('CREATED', 'PENDING') AND invoice_id IS NOT NULL;

-- SPLIT billing: the institution's share of a subscription's price (the
-- rider pays the rest). INSTITUTION_PAYS: the whole price.
ALTER TABLE subscriptions ADD COLUMN institution_share_cents bigint NOT NULL DEFAULT 0
  CHECK (institution_share_cents >= 0 AND institution_share_cents <= price_cents);

-- A subscription is invoiced once per billing period.
ALTER TABLE invoice_lines ADD COLUMN period daterange;
CREATE UNIQUE INDEX uq_invoice_lines_subscription_period ON invoice_lines (subscription_id, period)
  WHERE subscription_id IS NOT NULL;
CREATE INDEX ix_invoices_institution ON invoices (institution_id, created_at DESC);
CREATE SEQUENCE invoice_number_seq;

CREATE INDEX ix_trips_route_shift ON trips (route_shift_id);
CREATE INDEX ix_manifest_trip_stop ON trip_manifest (trip_id, stop_id);

-- Receipt, renewal and invoice notifications (am + en).
INSERT INTO notification_templates (code, channel, language, title, body)
SELECT m.code, c.channel::notif_channel, m.language::app_language, m.title, m.body
  FROM (VALUES
    ('payment.succeeded', 'en', 'Payment received', 'We received {{.Amount}} for {{.RiderName}}''s Efoy subscription ({{.Period}}). Receipt {{.Receipt}}.'),
    ('payment.succeeded', 'am', 'ክፍያ ደርሷል', 'ለ{{.RiderName}} የእፋይ ምዝገባ ({{.Period}}) {{.Amount}} ተቀብለናል። ደረሰኝ {{.Receipt}}።'),
    ('payment.failed', 'en', 'Payment not completed', 'Your payment of {{.Amount}} for {{.RiderName}} did not go through. Please try again in the Efoy app.'),
    ('payment.failed', 'am', 'ክፍያው አልተጠናቀቀም', 'ለ{{.RiderName}} የከፈሉት {{.Amount}} አልተሳካም። እባክዎ በእፋይ መተግበሪያ እንደገና ይሞክሩ።'),
    ('subscription.renewal_due', 'en', 'Subscription renewal due', '{{.RiderName}}''s Efoy subscription ends on {{.EndsOn}}. Renew for {{.Amount}} in the Efoy app to keep the seat.'),
    ('subscription.renewal_due', 'am', 'ምዝገባ የሚታደስበት ጊዜ ደርሷል', 'የ{{.RiderName}} የእፋይ ምዝገባ በ{{.EndsOn}} ያበቃል። መቀመጫውን ለማቆየት በእፋይ መተግበሪያ በ{{.Amount}} ያድሱ።'),
    ('subscription.past_due', 'en', 'Subscription payment overdue', '{{.RiderName}}''s Efoy subscription has ended. The seat is kept until {{.GraceEndsOn}}; renew for {{.Amount}} to keep it.'),
    ('subscription.past_due', 'am', 'የምዝገባ ክፍያ ዘግይቷል', 'የ{{.RiderName}} የእፋይ ምዝገባ አብቅቷል። መቀመጫው እስከ {{.GraceEndsOn}} ይቆያል፤ ለማቆየት በ{{.Amount}} ያድሱ።'),
    ('subscription.cancelled', 'en', 'Subscription cancelled', '{{.RiderName}}''s Efoy subscription was cancelled and the seat released: {{.Reason}}'),
    ('subscription.cancelled', 'am', 'ምዝገባ ተሰርዟል', 'የ{{.RiderName}} የእፋይ ምዝገባ ተሰርዞ መቀመጫው ተለቋል፦ {{.Reason}}'),
    ('invoice.issued', 'en', 'New Efoy invoice', 'Invoice {{.Number}} for {{.Amount}} is ready. It is due on {{.DueOn}}.'),
    ('invoice.issued', 'am', 'አዲስ የእፋይ ደረሰኝ', 'ቁጥር {{.Number}} የ{{.Amount}} ሂሳብ ተዘጋጅቷል። የሚከፈልበት ቀን {{.DueOn}} ነው።')
  ) AS m(code, language, title, body)
 CROSS JOIN (VALUES ('PUSH'), ('SMS'), ('IN_APP')) AS c(channel)
ON CONFLICT (code, channel, language) DO UPDATE SET title = EXCLUDED.title, body = EXCLUDED.body;

-- +goose Down
DELETE FROM notification_templates
 WHERE code IN ('payment.succeeded', 'payment.failed', 'subscription.renewal_due', 'subscription.past_due',
                'subscription.cancelled', 'invoice.issued');
DROP INDEX ix_manifest_trip_stop;
DROP INDEX ix_trips_route_shift;
DROP SEQUENCE invoice_number_seq;
DROP INDEX ix_invoices_institution;
DROP INDEX uq_invoice_lines_subscription_period;
ALTER TABLE invoice_lines DROP COLUMN period;
ALTER TABLE subscriptions DROP COLUMN institution_share_cents;
DROP INDEX uq_payment_intents_open_invoice;
DROP INDEX uq_payment_intents_open_subscription;
DROP INDEX ix_payment_intents_created;
DROP INDEX ix_payment_intents_invoice;
DROP INDEX ix_payment_intents_subscription;
DROP TRIGGER trg_payment_intents_updated ON payment_intents;
ALTER TABLE payment_intents
  DROP COLUMN updated_at, DROP COLUMN last_verified_at, DROP COLUMN reviewed_at, DROP COLUMN reviewed_by,
  DROP COLUMN bank_reference, DROP COLUMN receipt_key, DROP COLUMN payer_phone, DROP COLUMN renewal_quote,
  DROP COLUMN renewal_period, DROP COLUMN purpose;
