package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// resendEndpoint is Resend's documented transactional-email endpoint.
const resendEndpoint = "https://api.resend.com/emails"

// resendSender implements Sender against the Resend HTTP API. endpoint is
// broken out as a field (rather than always using resendEndpoint directly)
// so a test can point it at an httptest.Server without any network access.
type resendSender struct {
	apiKey      string
	fromAddress string
	fromName    string
	endpoint    string
	client      *http.Client
}

// NewResendSender returns a Sender backed by Resend. Resend was selected at
// planning time (RESEARCH.md Open Question 2): it needs no AWS sandbox exit
// and no SDK dependency, only a single HTTPS POST.
func NewResendSender(apiKey, fromAddress, fromName string) Sender {
	return &resendSender{
		apiKey:      apiKey,
		fromAddress: fromAddress,
		fromName:    fromName,
		endpoint:    resendEndpoint,
		client:      &http.Client{Timeout: 10 * time.Second},
	}
}

type resendEmailRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
	Text    string   `json:"text"`
}

// Send posts the message to Resend with a bearer-authenticated request. A
// non-2xx response is surfaced as an error carrying only the status code,
// never the response body, so a provider error message (which could carry
// account or billing detail) cannot flow into a user-facing response.
func (s *resendSender) Send(ctx context.Context, to, subject, html, text string) error {
	from := s.fromAddress
	if s.fromName != "" {
		from = fmt.Sprintf("%s <%s>", s.fromName, s.fromAddress)
	}
	payload, err := json.Marshal(resendEmailRequest{
		From:    from,
		To:      []string{to},
		Subject: subject,
		HTML:    html,
		Text:    text,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("mail: resend responded with status %d", resp.StatusCode)
	}
	return nil
}
