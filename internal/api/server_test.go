package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gorillaws "github.com/gorilla/websocket"

	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/broker"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/chatlog"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/crypto"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/merkle"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/models"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/ratelimit"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/service"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/storage"
)

func newTestServer() http.Handler {
	svc := service.New(chatlog.New(storage.NewInMemoryStorage()))
	return NewServer(svc).Handler()
}

// sendText signs a text message client-side and posts it, returning the
// created entry.
func sendText(t *testing.T, h http.Handler, chatID, text string) models.LogEntry {
	t.Helper()
	priv, pub, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	msg := models.NewMessage(chatID, "alice", pub, []byte(text), models.ContentTypeText, "", false)
	msg = crypto.SignMessage(msg, priv)
	return postMessage(t, h, "/chats/"+chatID+"/messages", msg, http.StatusCreated)
}

// sendAttachment signs an encrypted attachment client-side and posts it to
// path, returning the response status so callers can assert on rejections.
func sendAttachment(t *testing.T, h http.Handler, path, chatID string, contentType, filename string, data []byte) (models.LogEntry, int) {
	t.Helper()
	priv, pub, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	contentKey, err := crypto.NewContentKey()
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := crypto.Encrypt(contentKey, data)
	if err != nil {
		t.Fatal(err)
	}
	msg := models.NewMessage(chatID, "alice", pub, ciphertext, contentType, filename, true)
	msg = crypto.SignMessage(msg, priv)

	body, _ := json.Marshal(msg)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	h.ServeHTTP(rec, req)

	var entry models.LogEntry
	if rec.Code == http.StatusCreated {
		if err := json.Unmarshal(rec.Body.Bytes(), &entry); err != nil {
			t.Fatal(err)
		}
	}
	return entry, rec.Code
}

func postMessage(t *testing.T, h http.Handler, path string, msg models.SignedMessage, wantStatus int) models.LogEntry {
	t.Helper()
	body, _ := json.Marshal(msg)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	h.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("post %s: want %d, got %d: %s", path, wantStatus, rec.Code, rec.Body)
	}
	var entry models.LogEntry
	if wantStatus == http.StatusCreated {
		if err := json.Unmarshal(rec.Body.Bytes(), &entry); err != nil {
			t.Fatal(err)
		}
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

func TestVerifyEndpoint(t *testing.T) {
	h := newTestServer()
	sendText(t, h, "c1", "hello")
	sendText(t, h, "c1", "world")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/chats/c1/verify", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
	}
	var res chatlog.VerifyResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if !res.Valid || res.Entries != 2 {
		t.Fatalf("unexpected result: %+v", res)
	}

	// An empty chat is trivially valid with zero entries.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/chats/empty/verify", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 for an empty chat, got %d", rec.Code)
	}
	res = chatlog.VerifyResult{}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if !res.Valid || res.Entries != 0 {
		t.Fatalf("unexpected result for empty chat: %+v", res)
	}
}

func TestSyncEndpoint(t *testing.T) {
	h := newTestServer()

	// Before any snapshot is sealed, sync returns every entry and no snapshot.
	sent := sendText(t, h, "c1", "hello")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/chats/c1/sync", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
	}
	var bundle chatlog.SyncBundle
	if err := json.Unmarshal(rec.Body.Bytes(), &bundle); err != nil {
		t.Fatal(err)
	}
	if bundle.Snapshot != nil {
		t.Fatalf("expected no snapshot yet, got %+v", bundle.Snapshot)
	}
	if len(bundle.NewEntries) != 1 || bundle.CurrentHash != sent.EntryHash {
		t.Fatalf("unexpected bundle: %+v", bundle)
	}

	// Fill the window to seal a snapshot; sync now reports it plus the tail.
	for i := 1; i < models.SnapshotInterval; i++ {
		sendText(t, h, "c1", "msg")
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/chats/c1/sync", nil))
	bundle = chatlog.SyncBundle{}
	if err := json.Unmarshal(rec.Body.Bytes(), &bundle); err != nil {
		t.Fatal(err)
	}
	if bundle.Snapshot == nil {
		t.Fatal("expected a sealed snapshot after filling the window")
	}
	if len(bundle.NewEntries) != 0 {
		t.Fatalf("expected no entries after the snapshot's tip, got %d", len(bundle.NewEntries))
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

func TestSendRejectsMissingSignature(t *testing.T) {
	h := newTestServer()
	priv, pub, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	msg := models.NewMessage("c1", "alice", pub, []byte("hi"), models.ContentTypeText, "", false)
	_ = priv // intentionally not signing

	body, _ := json.Marshal(msg)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/chats/c1/messages", bytes.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for unsigned message, got %d: %s", rec.Code, rec.Body)
	}
}

func TestSendRejectsMessageSignedForAnotherChat(t *testing.T) {
	h := newTestServer()
	priv, pub, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	msg := models.NewMessage("chat-a", "alice", pub, []byte("hi"), models.ContentTypeText, "", false)
	msg = crypto.SignMessage(msg, priv)

	// POSTed to chat-b: the handler rebinds chat_id from the path, so the
	// signature (computed over chat-a) no longer verifies.
	body, _ := json.Marshal(msg)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/chats/chat-b/messages", bytes.NewReader(body)))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 for cross-chat signature, got %d: %s", rec.Code, rec.Body)
	}
}

func TestSendPhoto(t *testing.T) {
	h := newTestServer()
	photo := []byte("\xff\xd8\xff\xe0 fake JPEG bytes")

	entry, status := sendAttachment(t, h, "/chats/c1/photos", "c1", "image/jpeg", "cat.jpg", photo)
	if status != http.StatusCreated {
		t.Fatalf("want 201, got %d", status)
	}
	if !entry.Message.Encrypted || entry.Message.ContentType != "image/jpeg" {
		t.Fatalf("unexpected message: %+v", entry.Message)
	}
}

func TestSendPhotoRejectsOversize(t *testing.T) {
	h := newTestServer()
	big := make([]byte, service.MaxPhotoBytes+1)

	_, status := sendAttachment(t, h, "/chats/c1/photos", "c1", "image/png", "", big)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 for oversize photo, got %d", status)
	}
}

func TestSendVideo(t *testing.T) {
	h := newTestServer()
	video := []byte("\x00\x00\x00\x18ftypmp42 fake MP4 bytes")

	entry, status := sendAttachment(t, h, "/chats/c1/videos", "c1", "video/mp4", "clip.mp4", video)
	if status != http.StatusCreated {
		t.Fatalf("want 201, got %d", status)
	}
	if !entry.Message.Encrypted || entry.Message.ContentType != "video/mp4" {
		t.Fatalf("unexpected message: %+v", entry.Message)
	}
}

func TestSendVideoRejectsOversize(t *testing.T) {
	h := newTestServer()
	big := make([]byte, service.MaxVideoBytes+1)

	_, status := sendAttachment(t, h, "/chats/c1/videos", "c1", "video/mp4", "", big)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 for oversize video, got %d", status)
	}
}

func TestStreamDeliversEntryAppendedEvent(t *testing.T) {
	svc := service.New(chatlog.New(storage.NewInMemoryStorage(), chatlog.WithBroker(broker.NewInMemory())))
	srv := httptest.NewServer(NewServer(svc).Handler())
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):] + "/chats/c1/ws"
	conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	defer conn.Close()

	// Send a message via the normal HTTP path once the socket is open.
	go func() {
		priv, pub, _ := crypto.GenerateKeyPair()
		msg := models.NewMessage("c1", "alice", pub, []byte("hi"), models.ContentTypeText, "", false)
		msg = crypto.SignMessage(msg, priv)
		body, _ := json.Marshal(msg)
		http.Post(srv.URL+"/chats/c1/messages", "application/json", bytes.NewReader(body))
	}()

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var evt struct {
		Kind      string `json:"kind"`
		ChatID    string `json:"chat_id"`
		Sequence  uint64 `json:"sequence"`
		EntryHash string `json:"entry_hash"`
	}
	if err := conn.ReadJSON(&evt); err != nil {
		t.Fatalf("read event: %v", err)
	}
	if evt.Kind != "entry_appended" || evt.ChatID != "c1" || evt.EntryHash == "" {
		t.Fatalf("unexpected event: %+v", evt)
	}
}

func TestStreamIgnoresOtherChats(t *testing.T) {
	svc := service.New(chatlog.New(storage.NewInMemoryStorage(), chatlog.WithBroker(broker.NewInMemory())))
	srv := httptest.NewServer(NewServer(svc).Handler())
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):] + "/chats/c1/ws"
	conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	defer conn.Close()

	priv, pub, _ := crypto.GenerateKeyPair()
	msg := models.NewMessage("other-chat", "alice", pub, []byte("hi"), models.ContentTypeText, "", false)
	msg = crypto.SignMessage(msg, priv)
	body, _ := json.Marshal(msg)
	if resp, err := http.Post(srv.URL+"/chats/other-chat/messages", "application/json", bytes.NewReader(body)); err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("seed message: err=%v resp=%v", err, resp)
	}

	conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	var evt map[string]any
	if err := conn.ReadJSON(&evt); err == nil {
		t.Fatalf("expected no event for a different chat, got %+v", evt)
	}
}

func TestStreamAcceptsCrossOriginHandshake(t *testing.T) {
	svc := service.New(chatlog.New(storage.NewInMemoryStorage(), chatlog.WithBroker(broker.NewInMemory())))
	srv := httptest.NewServer(NewServer(svc).Handler())
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):] + "/chats/c1/ws"
	header := http.Header{"Origin": {"http://a-completely-different-origin.example:9999"}}
	conn, resp, err := gorillaws.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial ws with foreign Origin: %v (status %v)", err, resp)
	}
	defer conn.Close()
}

func newRateLimitedTestServer(burst int) http.Handler {
	svc := service.New(chatlog.New(storage.NewInMemoryStorage()))
	limiter := ratelimit.New(1, burst, time.Minute)
	return NewServer(svc, WithRateLimit(limiter)).Handler()
}

func getWithIP(h http.Handler, path, remoteAddr string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = remoteAddr
	h.ServeHTTP(rec, req)
	return rec
}

func TestRateLimitReturns429WhenBurstExceeded(t *testing.T) {
	h := newRateLimitedTestServer(2)

	for i := 0; i < 2; i++ {
		rec := getWithIP(h, "/chats/c1/messages", "203.0.113.1:1111")
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d within burst: want 200, got %d", i, rec.Code)
		}
	}
	rec := getWithIP(h, "/chats/c1/messages", "203.0.113.1:1111")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429 beyond burst, got %d: %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("expected a Retry-After header on 429")
	}
}

func TestRateLimitIsPerClientIP(t *testing.T) {
	h := newRateLimitedTestServer(1)

	if rec := getWithIP(h, "/chats/c1/messages", "203.0.113.1:1111"); rec.Code != http.StatusOK {
		t.Fatalf("client A's first request: want 200, got %d", rec.Code)
	}
	if rec := getWithIP(h, "/chats/c1/messages", "203.0.113.2:2222"); rec.Code != http.StatusOK {
		t.Fatalf("client B's first request: want 200, got %d (should have its own bucket)", rec.Code)
	}
	if rec := getWithIP(h, "/chats/c1/messages", "203.0.113.1:1111"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("client A's second request: want 429, got %d", rec.Code)
	}
}

func TestHealthzExemptFromRateLimit(t *testing.T) {
	h := newRateLimitedTestServer(1)

	for i := 0; i < 5; i++ {
		rec := getWithIP(h, "/healthz", "203.0.113.1:1111")
		if rec.Code != http.StatusOK {
			t.Fatalf("healthz request %d: want 200, got %d", i, rec.Code)
		}
	}
}
