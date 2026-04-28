package email_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emiliopalmerini/chianti/platform/email"
)

// ADR-017 #4: Send forwards attachments to the mailer; the capture records
// filename, content-type, and body length.
func TestSendForwardsAttachments(t *testing.T) {
	cap := &attachmentCapture{}
	err := cap.Send(context.Background(), email.Message{
		To:      []string{"to@example.com"},
		Subject: "Test",
		HTML:    "<p>body</p>",
		Attachments: []email.Attachment{
			{Filename: "a.pdf", ContentType: "application/pdf", Body: []byte("PDFBYTES")},
			{Filename: "b.pdf", ContentType: "application/pdf", Body: []byte("MORE")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cap.last.Attachments) != 2 {
		t.Fatalf("attachments len = %d, want 2", len(cap.last.Attachments))
	}
	if cap.last.Attachments[0].Filename != "a.pdf" {
		t.Errorf("filename = %q", cap.last.Attachments[0].Filename)
	}
	if cap.last.Attachments[0].ContentType != "application/pdf" {
		t.Errorf("ct = %q", cap.last.Attachments[0].ContentType)
	}
	if len(cap.last.Attachments[0].Body) != 8 {
		t.Errorf("body len = %d", len(cap.last.Attachments[0].Body))
	}
}

// ADR-017 #5: existing zero-attachment calls compile and run unchanged.
func TestSendZeroAttachmentsStillWorks(t *testing.T) {
	cap := &attachmentCapture{}
	err := cap.Send(context.Background(), email.Message{
		To: []string{"to@example.com"}, Subject: "T",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cap.last.Attachments != nil && len(cap.last.Attachments) != 0 {
		t.Errorf("attachments should be empty: %+v", cap.last.Attachments)
	}
}

// Resend adapter base64-encodes each attachment's body in the POST payload.
func TestResendSenderEncodesAttachments(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	defer srv.Close()

	sender, err := email.NewResendSenderWithEndpoint("key", "from@x", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	err = sender.Send(context.Background(), email.Message{
		To:      []string{"to@y"},
		Subject: "s",
		HTML:    "h",
		Attachments: []email.Attachment{
			{Filename: "doc.pdf", ContentType: "application/pdf", Body: []byte("hello")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, ok := captured["attachments"].([]any)
	if !ok || len(raw) != 1 {
		t.Fatalf("attachments not in payload: %+v", captured)
	}
	first, ok := raw[0].(map[string]any)
	if !ok {
		t.Fatalf("attachment is not an object: %+v", raw[0])
	}
	if first["filename"] != "doc.pdf" {
		t.Errorf("filename = %v", first["filename"])
	}
	// base64("hello") = "aGVsbG8="
	if first["content"] != "aGVsbG8=" {
		t.Errorf("content = %v, want base64 of hello", first["content"])
	}
}

type attachmentCapture struct {
	last email.Message
}

func (c *attachmentCapture) Send(_ context.Context, msg email.Message) error {
	c.last = msg
	return nil
}
