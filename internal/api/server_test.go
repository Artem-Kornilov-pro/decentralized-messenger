package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/chatlog"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/crypto"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/merkle"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/models"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/service"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/storage"
)

func newTestServer() http.Handler {
	svc := service.New(chatlog.New(storage.NewInMemoryStorage()))
	return NewServer(svc).Handler()
}

// sendText posts a signed text message and returns the created entry.
func sendText(t *testing.T, h http.Handler, chatID, text string) models.LogEntry {
	t.Helper()
	priv, pub, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(sendRequest{
		SenderID:   "alice",
		PublicKey:  pub,
		PrivateKey: priv,
		Text:       text,
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/chats/"+chatID+"/messages", bytes.NewReader(body))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("send: want 201, got %d: %s", rec.Code, rec.Body)
	}
	var entry models.LogEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	return entry
}

func TestHistoryPaginates(t *testing.T) {
	h := newTestServer()
	for i := 0; i < 3; i++ {
		sendText(t, h, "c1", "msg")
	}

	// First page of 2 should advance the cursor.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/chats/c1/messages?from=0&limit=2", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var page historyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 2 || page.NextFrom == nil || *page.NextFrom != 2 {
		t.Fatalf("unexpected first page: %d msgs, next=%v", len(page.Messages), page.NextFrom)
	}

	// Final page should not advance the cursor.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/chats/c1/messages?from=2&limit=2", nil))
	json.Unmarshal(rec.Body.Bytes(), &page)
	if len(page.Messages) != 1 || page.NextFrom != nil {
		t.Fatalf("unexpected final page: %d msgs, next=%v", len(page.Messages), page.NextFrom)
	}
}

func TestGetMessageBySequence(t *testing.T) {
	h := newTestServer()
	sent := sendText(t, h, "c1", "hello")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/chats/c1/messages/0", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var got models.LogEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.EntryHash != sent.EntryHash || string(got.Message.Content) != "hello" {
		t.Fatalf("fetched entry mismatch: %+v", got)
	}
}

func TestGetMessageNotFound(t *testing.T) {
	h := newTestServer()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/chats/c1/messages/99", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rec.Code)
	}
}

func TestProofEndpoint(t *testing.T) {
	h := newTestServer()

	// Before a snapshot is sealed, a proof is unavailable.
	sendText(t, h, "c1", "first")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/chats/c1/messages/0/proof", nil))
	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409 before snapshot, got %d", rec.Code)
	}

	// Fill the window to seal a snapshot, then a proof verifies.
	for i := 1; i < models.SnapshotInterval; i++ {
		sendText(t, h, "c1", "msg")
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/chats/c1/messages/7/proof", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 after snapshot, got %d: %s", rec.Code, rec.Body)
	}
	var proof chatlog.InclusionProof
	if err := json.Unmarshal(rec.Body.Bytes(), &proof); err != nil {
		t.Fatal(err)
	}
	if !merkle.VerifyProof(proof.EntryHash, proof.Proof, proof.MerkleRoot) {
		t.Fatal("returned proof failed to verify")
	}
}

func TestVerifyMessageEndpoint(t *testing.T) {
	h := newTestServer()
	sendText(t, h, "c1", "hello")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/chats/c1/messages/0/verify", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var res chatlog.MessageVerification
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if !res.Valid || res.Sequence != 0 {
		t.Fatalf("unexpected result: %+v", res)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/chats/c1/messages/9/verify", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 for missing message, got %d", rec.Code)
	}
}

func TestGetMessageBadSequence(t *testing.T) {
	h := newTestServer()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/chats/c1/messages/abc", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}
