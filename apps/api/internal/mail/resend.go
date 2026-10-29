package mail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const resendURL = "https://api.resend.com/emails"

// Resend sends plain-text email through the Resend HTTP API (production).
type Resend struct {
	APIKey string
	From   string
	URL    string // overrides the Resend endpoint; used in tests
}

func (m Resend) Send(to, subject, body string) error {
	payload, err := json.Marshal(map[string]any{
		"from": m.From, "to": []string{to}, "subject": subject, "text": body,
	})
	if err != nil {
		return err
	}
	url := m.URL
	if url == "" {
		url = resendURL
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+m.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("mail: resend: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("mail: resend: %s: %s", resp.Status, b)
	}
	return nil
}
