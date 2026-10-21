package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ExpoURL is Expo's push service.
const ExpoURL = "https://exp.host/--/api/v2/push/send"

// expoBatch is the most messages Expo takes in one request.
const expoBatch = 100

// TokenStore is what ExpoSender reads and cleans.
type TokenStore interface {
	Tokens(ctx context.Context, userID uuid.UUID) ([]string, error)
	RemoveTokens(ctx context.Context, tokens []string) error
}

// ExpoSender sends through Expo's push service. Best effort: a failure is
// logged and dropped, and a token Expo reports as DeviceNotRegistered (the
// app was deleted or notifications were turned off for good) is removed.
type ExpoSender struct {
	URL         string
	AccessToken string // optional EXPO_ACCESS_TOKEN
	Client      *http.Client
	Tokens      TokenStore
	Log         *slog.Logger
}

// NewExpoSender builds an ExpoSender with a short timeout.
func NewExpoSender(tokens TokenStore, accessToken string, log *slog.Logger) *ExpoSender {
	if log == nil {
		log = slog.Default()
	}
	return &ExpoSender{
		URL: ExpoURL, AccessToken: accessToken, Tokens: tokens, Log: log,
		Client: &http.Client{Timeout: 10 * time.Second},
	}
}

type expoMessage struct {
	To    string            `json:"to"`
	Title string            `json:"title"`
	Body  string            `json:"body"`
	Data  map[string]string `json:"data,omitempty"`
	Sound string            `json:"sound,omitempty"`
}

type expoTicket struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Details struct {
		Error string `json:"error"`
	} `json:"details"`
}

type expoResponse struct {
	Data []expoTicket `json:"data"`
}

// Send implements Sender.
func (s *ExpoSender) Send(ctx context.Context, msgs []Message) {
	var out []expoMessage
	for _, m := range msgs {
		tokens, err := s.Tokens.Tokens(ctx, m.UserID)
		if err != nil {
			s.Log.Warn("push: reading tokens failed", "error", err)
			continue
		}
		for _, t := range tokens {
			out = append(out, expoMessage{To: t, Title: m.Title, Body: m.Body, Data: m.Data, Sound: "default"})
		}
	}
	for start := 0; start < len(out); start += expoBatch {
		end := min(start+expoBatch, len(out))
		s.sendBatch(ctx, out[start:end])
	}
}

func (s *ExpoSender) sendBatch(ctx context.Context, batch []expoMessage) {
	dead, err := s.post(ctx, batch)
	if err != nil {
		s.Log.Warn("push: expo send failed", "messages", len(batch), "error", err)
	}
	if len(dead) > 0 {
		if err := s.Tokens.RemoveTokens(ctx, dead); err != nil {
			s.Log.Warn("push: removing dead tokens failed", "error", err)
		}
	}
}

// post sends one batch and answers the tokens Expo says are gone.
func (s *ExpoSender) post(ctx context.Context, batch []expoMessage) ([]string, error) {
	body, err := json.Marshal(batch)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if s.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.AccessToken)
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expo answered %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	var parsed expoResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("expo answer not readable: %w", err)
	}
	var dead []string
	failed := 0
	for i, t := range parsed.Data {
		if t.Status != "error" {
			continue
		}
		failed++
		if t.Details.Error == "DeviceNotRegistered" && i < len(batch) {
			dead = append(dead, batch[i].To)
		}
	}
	if failed > 0 {
		s.Log.Warn("push: expo refused some messages", "failed", failed, "removed", len(dead))
	}
	return dead, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
