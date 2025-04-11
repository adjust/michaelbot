package bot

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// SlackVerifier verifies that requests are coming from Slack
type SlackVerifier struct {
	signingSecret string
}

// NewSlackVerifier creates a new SlackVerifier with the given signing secret
func NewSlackVerifier(signingSecret string) *SlackVerifier {
	return &SlackVerifier{
		signingSecret: signingSecret,
	}
}

// Verify checks if the request is actually coming from Slack
// Returns true if the request is verified, false otherwise
func (v *SlackVerifier) Verify(r *http.Request) bool {
	timestamp := r.Header.Get("X-Slack-Request-Timestamp")
	signature := r.Header.Get("X-Slack-Signature")

	// No headers, no verification
	if timestamp == "" || signature == "" {
		return false
	}

	// Verify the request is not too old (prevent replay attacks)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || time.Now().Unix()-ts > 60*5 {
		return false
	}

	// Read the body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return false
	}
	// Replace the body so it can be read again
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	// Create the signature base string
	baseString := fmt.Sprintf("v0:%s:%s", timestamp, string(body))

	// Create the signature using the signing secret
	h := hmac.New(sha256.New, []byte(v.signingSecret))
	h.Write([]byte(baseString))
	calculatedSignature := fmt.Sprintf("v0=%s", hex.EncodeToString(h.Sum(nil)))

	// Compare signatures
	return hmac.Equal([]byte(signature), []byte(calculatedSignature))
}
