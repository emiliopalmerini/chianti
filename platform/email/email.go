// Package email provides an outbound email Sender abstraction and three
// implementations: a Resend HTTP adapter, a dev-override wrapper that
// redirects all recipients to a single address, and a noop logger-only
// sender for dev when no API key is configured.
package email

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Attachment is a single file attached to an outbound Message. Body is the
// raw bytes; the Resend adapter base64-encodes them automatically.
type Attachment struct {
	Filename    string
	ContentType string
	Body        []byte
}

type Message struct {
	To          []string
	Subject     string
	HTML        string
	Text        string
	Attachments []Attachment
}

type Sender interface {
	Send(ctx context.Context, msg Message) error
}

const resendAPIURL = "https://api.resend.com/emails"

type resendSender struct {
	apiKey   string
	from     string
	endpoint string
	client   *http.Client
}

func NewResendSender(apiKey, from string) (Sender, error) {
	return NewResendSenderWithEndpoint(apiKey, from, resendAPIURL)
}

// NewResendSenderWithEndpoint is test-friendly: it lets a fake HTTP server
// stand in for api.resend.com. Production callers use NewResendSender.
func NewResendSenderWithEndpoint(apiKey, from, endpoint string) (Sender, error) {
	if apiKey == "" {
		return nil, errors.New("email: api key required")
	}
	if from == "" {
		return nil, errors.New("email: from address required")
	}
	if endpoint == "" {
		endpoint = resendAPIURL
	}
	return &resendSender{
		apiKey:   apiKey,
		from:     from,
		endpoint: endpoint,
		client:   &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (r *resendSender) Send(ctx context.Context, msg Message) error {
	payload := map[string]any{
		"from":    r.from,
		"to":      msg.To,
		"subject": msg.Subject,
		"html":    msg.HTML,
	}
	if msg.Text != "" {
		payload["text"] = msg.Text
	}
	if len(msg.Attachments) > 0 {
		out := make([]map[string]any, 0, len(msg.Attachments))
		for _, a := range msg.Attachments {
			entry := map[string]any{
				"filename": a.Filename,
				"content":  base64.StdEncoding.EncodeToString(a.Body),
			}
			if a.ContentType != "" {
				entry["content_type"] = a.ContentType
			}
			out = append(out, entry)
		}
		payload["attachments"] = out
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("email: marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("email: request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("email: send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("email: send failed (status %d): %s", resp.StatusCode, string(b))
	}
	return nil
}

type devOverrideSender struct {
	inner    Sender
	override string
	logger   *slog.Logger
}

// NewDevOverride wraps inner so every Send has its recipients replaced with
// override. The original recipients are logged at warn level for traceability.
func NewDevOverride(inner Sender, override string) Sender {
	return &devOverrideSender{inner: inner, override: override, logger: slog.Default()}
}

func (d *devOverrideSender) Send(ctx context.Context, msg Message) error {
	d.logger.Warn("dev email override active", "original_to", msg.To, "override_to", d.override)
	msg.To = []string{d.override}
	return d.inner.Send(ctx, msg)
}

type noopSender struct {
	logger *slog.Logger
}

// NewNoop returns a Sender that logs and drops. Used in dev when
// RESEND_API_KEY is unset; refuses to be instantiated in production-shaped
// flows because config validation blocks an empty API key there.
func NewNoop(logger *slog.Logger) Sender {
	if logger == nil {
		logger = slog.Default()
	}
	return &noopSender{logger: logger}
}

func (n *noopSender) Send(_ context.Context, msg Message) error {
	n.logger.Info("noop email sender", "to", msg.To, "subject", msg.Subject)
	return nil
}
