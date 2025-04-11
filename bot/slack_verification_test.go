package bot

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"
)

// Helper function to create a valid signature for testing
func createTestSignature(secret, timestamp string, body []byte) string {
	baseString := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(baseString))
	return fmt.Sprintf("v0=%s", hex.EncodeToString(h.Sum(nil)))
}

func TestSlackVerifier_Verify_ValidSignature(t *testing.T) {
	secret := "testsecret"
	verifier := NewSlackVerifier(secret)

	body := []byte("test=body")
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := createTestSignature(secret, timestamp, body)

	req, _ := http.NewRequest("POST", "/", bytes.NewBuffer(body))
	req.Header.Set("X-Slack-Request-Timestamp", timestamp)
	req.Header.Set("X-Slack-Signature", signature)

	if !verifier.Verify(req) {
		t.Errorf("Verify() = false, want true for valid signature")
	}
}

func TestSlackVerifier_Verify_InvalidSignature(t *testing.T) {
	secret := "testsecret"
	verifier := NewSlackVerifier(secret)

	body := []byte("test=body")
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	invalidSignature := "v0=invalidsignature"

	req, _ := http.NewRequest("POST", "/", bytes.NewBuffer(body))
	req.Header.Set("X-Slack-Request-Timestamp", timestamp)
	req.Header.Set("X-Slack-Signature", invalidSignature)

	if verifier.Verify(req) {
		t.Errorf("Verify() = true, want false for invalid signature")
	}
}

func TestSlackVerifier_Verify_MissingTimestampHeader(t *testing.T) {
	secret := "testsecret"
	verifier := NewSlackVerifier(secret)

	body := []byte("test=body")
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := createTestSignature(secret, timestamp, body)

	req, _ := http.NewRequest("POST", "/", bytes.NewBuffer(body))
	// Missing X-Slack-Request-Timestamp
	req.Header.Set("X-Slack-Signature", signature)

	if verifier.Verify(req) {
		t.Errorf("Verify() = true, want false when timestamp header is missing")
	}
}

func TestSlackVerifier_Verify_MissingSignatureHeader(t *testing.T) {
	secret := "testsecret"
	verifier := NewSlackVerifier(secret)

	body := []byte("test=body")
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	req, _ := http.NewRequest("POST", "/", bytes.NewBuffer(body))
	req.Header.Set("X-Slack-Request-Timestamp", timestamp)
	// Missing X-Slack-Signature

	if verifier.Verify(req) {
		t.Errorf("Verify() = true, want false when signature header is missing")
	}
}

func TestSlackVerifier_Verify_OldTimestamp(t *testing.T) {
	secret := "testsecret"
	verifier := NewSlackVerifier(secret)

	body := []byte("test=body")
	// Timestamp from 6 minutes ago
	oldTimestamp := strconv.FormatInt(time.Now().Add(-6*time.Minute).Unix(), 10)
	signature := createTestSignature(secret, oldTimestamp, body)

	req, _ := http.NewRequest("POST", "/", bytes.NewBuffer(body))
	req.Header.Set("X-Slack-Request-Timestamp", oldTimestamp)
	req.Header.Set("X-Slack-Signature", signature)

	if verifier.Verify(req) {
		t.Errorf("Verify() = true, want false for old timestamp")
	}
}

func TestSlackVerifier_Verify_FutureTimestamp(t *testing.T) {
	secret := "testsecret"
	verifier := NewSlackVerifier(secret)

	body := []byte("test=body")
	// Timestamp 6 minutes in the future (should still be considered recent enough)
	futureTimestamp := strconv.FormatInt(time.Now().Add(6*time.Minute).Unix(), 10)
	signature := createTestSignature(secret, futureTimestamp, body)

	req, _ := http.NewRequest("POST", "/", bytes.NewBuffer(body))
	req.Header.Set("X-Slack-Request-Timestamp", futureTimestamp)
	req.Header.Set("X-Slack-Signature", signature)

	if !verifier.Verify(req) {
		t.Errorf("Verify() = false, want true for future timestamp (within reasonable clock skew)")
	}
}

func TestSlackVerifier_Verify_EmptyBody(t *testing.T) {
	secret := "testsecret"
	verifier := NewSlackVerifier(secret)

	emptyBody := []byte("")
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := createTestSignature(secret, timestamp, emptyBody)

	req, _ := http.NewRequest("POST", "/", bytes.NewBuffer(emptyBody))
	req.Header.Set("X-Slack-Request-Timestamp", timestamp)
	req.Header.Set("X-Slack-Signature", signature)

	if !verifier.Verify(req) {
		t.Errorf("Verify() = false, want true for valid signature with empty body")
	}
}
