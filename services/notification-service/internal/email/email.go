// Package email is the swap point for a real provider (SES, SendGrid,
// Postmark). No SMTP credentials exist in this local/demo deployment, so
// Sender's only implementation here logs what would have been sent --
// same "build locally first" pattern as payment-service's simulated
// charges and stream-service's MinIO-standing-in-for-S3: the interface
// callers integrate against doesn't change when the real implementation
// lands behind it.
package email

import "log/slog"

type Sender interface {
	Send(to, subject, body string) error
}

type ConsoleSender struct{}

func (ConsoleSender) Send(to, subject, body string) error {
	slog.Info("email (not actually sent -- no provider configured)", "to", to, "subject", subject, "body", body)
	return nil
}
