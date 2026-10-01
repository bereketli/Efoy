// Package notification renders templates and delivers push, SMS and in-app
// messages. The SMS gateway adapters (AfroMessage or GeezSMS) and the
// notification worker arrive on day 5; until then ConsoleSMS stands in.
package notification

import (
	"context"
	"log/slog"
)

// ConsoleSMS is the development SMS adapter: it writes each message to the
// log instead of sending it. Config refuses it in prod. On staging, read OTP
// codes from the core-api logs until a real gateway is configured.
type ConsoleSMS struct {
	log *slog.Logger
}

func NewConsoleSMS(log *slog.Logger) *ConsoleSMS {
	return &ConsoleSMS{log: log}
}

func (c *ConsoleSMS) SendSMS(ctx context.Context, to, body string) error {
	c.log.InfoContext(ctx, "sms (console fake, not sent)", slog.String("to", to), slog.String("body", body))
	return nil
}
