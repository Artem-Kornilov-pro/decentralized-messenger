// Package api exposes the messenger over HTTP/JSON.
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/chatlog"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/crypto"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/models"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/service"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/storage"
)

// pingInterval is how often handleStream sends a WebSocket ping to keep the
// connection alive through intermediary proxies.
const pingInterval = 20 * time.Second

var upgrader = websocket.Upgrader{}

// Pagination defaults for the history endpoint.
const (
	defaultHistoryLimit = 50
	maxHistoryLimit     = 200
)

// Server wires the Messenger service to HTTP handlers.
type Server struct {
	svc *service.Messenger
}

// NewServer returns an HTTP server for the given service.
func NewServer(svc *service.Messenger) *Server {
	return &Server{svc: svc}
}

// Handler returns the configured HTTP mux.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("POST /keys", s.HandleGenerateKeys)
	mux.HandleFunc("POST /keys/content", s.handleGenerateContentKey)
	mux.HandleFunc("POST /chats/{chatID}/messages", s.handleSend)
	mux.HandleFunc("GET /chats/{chatID}/messages", s.handleHistory)
	mux.HandleFunc("GET /chats/{chatID}/messages/{sequence}", s.handleMessage)
	mux.HandleFunc("GET /chats/{chatID}/messages/{sequence}/proof", s.handleProof)
	mux.HandleFunc("GET /chats/{chatID}/messages/{sequence}/verify", s.handleVerifyMessage)
	mux.HandleFunc("POST /chats/{chatID}/photos", s.handleSendPhoto)
	mux.HandleFunc("POST /chats/{chatID}/videos", s.handleSendVideo)
	mux.HandleFunc("GET /chats/{chatID}/verify", s.handleVerify)
	mux.HandleFunc("GET /chats/{chatID}/sync", s.handleSync)
	mux.HandleFunc("GET /chats/{chatID}/ws", s.handleStream)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// decodeSignedMessage reads a models.SignedMessage from the request body and
// binds it to the chat in the URL path: the chat_id is taken from the path,
// not the body, so a message signed for a different chat fails verification
// downstream instead of being silently accepted under the wrong chat.
func decodeSignedMessage(r *http.Request, chatID string) (models.SignedMessage, error) {
	var msg models.SignedMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		return models.SignedMessage{}, errors.New("invalid JSON body")
	}
	if msg.SenderID == "" || len(msg.PublicKey) == 0 || len(msg.Signature) == 0 {
		return models.SignedMessage{}, errors.New("sender_id, public_key and signature are required")
	}
	msg.ChatID = chatID
	return msg, nil
}

// handleSend appends a message the client has already signed locally (see
// models.NewMessage and crypto.SignMessage). The server never sees a private
// key — only the resulting signature.
func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	msg, err := decodeSignedMessage(r, r.PathValue("chatID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	entry, err := s.svc.Submit(msg, 0)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, entry)
}

func (s *Server) handleSendPhoto(w http.ResponseWriter, r *http.Request) {
	msg, err := decodeSignedMessage(r, r.PathValue("chatID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	entry, err := s.svc.Submit(msg, service.MaxPhotoBytes)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, entry)
}

func (s *Server) handleSendVideo(w http.ResponseWriter, r *http.Request) {
	msg, err := decodeSignedMessage(r, r.PathValue("chatID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	entry, err := s.svc.Submit(msg, service.MaxVideoBytes)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, entry)
}

// historyResponse is the paginated history payload. NextFrom is the sequence to
// pass as ?from= on the next request, or null when the end has been reached.
type historyResponse struct {
	Messages []models.LogEntry `json:"messages"`
	NextFrom *uint64           `json:"next_from"`
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	chatID := r.PathValue("chatID")

	from, err := parseUintQuery(r, "from", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid 'from' parameter")
		return
	}
	limit, err := parseUintQuery(r, "limit", defaultHistoryLimit)
	if err != nil || limit == 0 {
		writeError(w, http.StatusBadRequest, "invalid 'limit' parameter")
		return
	}
	if limit > maxHistoryLimit {
		limit = maxHistoryLimit
	}

	entries, err := s.svc.History(chatID, from, int(limit))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := historyResponse{Messages: entries}
	// A full page implies there may be more; advance the cursor past the last.
	if uint64(len(entries)) == limit && len(entries) > 0 {
		next := entries[len(entries)-1].Sequence + 1
		resp.NextFrom = &next
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleMessage(w http.ResponseWriter, r *http.Request) {
	chatID := r.PathValue("chatID")
	sequence, err := strconv.ParseUint(r.PathValue("sequence"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "sequence must be a non-negative integer")
		return
	}

	entry, err := s.svc.Message(chatID, sequence)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "message not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) handleVerifyMessage(w http.ResponseWriter, r *http.Request) {
	chatID := r.PathValue("chatID")
	sequence, err := strconv.ParseUint(r.PathValue("sequence"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "sequence must be a non-negative integer")
		return
	}

	result, err := s.svc.VerifyMessage(chatID, sequence)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "message not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleProof(w http.ResponseWriter, r *http.Request) {
	chatID := r.PathValue("chatID")
	sequence, err := strconv.ParseUint(r.PathValue("sequence"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "sequence must be a non-negative integer")
		return
	}

	proof, err := s.svc.ProveInclusion(chatID, sequence)
	if errors.Is(err, chatlog.ErrNotSnapshotted) {
		writeError(w, http.StatusConflict, "no snapshot yet covers this message (snapshots seal every 100 messages)")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, proof)
}

// parseUintQuery reads an unsigned integer query parameter, returning fallback
// when the parameter is absent.
func parseUintQuery(r *http.Request, key string, fallback uint64) (uint64, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback, nil
	}
	return strconv.ParseUint(raw, 10, 64)
}

func (s *Server) handleGenerateContentKey(w http.ResponseWriter, _ *http.Request) {
	key, err := crypto.NewContentKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string][]byte{"content_key": key})
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	result, err := s.svc.Verify(r.PathValue("chatID"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	bundle, err := s.svc.Sync(r.PathValue("chatID"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, bundle)
}

// handleStream upgrades to a WebSocket and pushes log events (new entries,
// sealed snapshots) for chatID as they happen, so clients don't have to poll
// GET /chats/{chatID}/messages. Events are a notification only — clients
// fetch the actual message via the existing REST endpoints.
//
// Subscribe() delivers every chat's events; filtering by ChatID happens here
// rather than via per-chat broker routing. That's fine at this project's
// scale — revisit with per-chat routing keys if fan-out volume ever matters.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	chatID := r.PathValue("chatID")

	// Subscribe before upgrading so the subscription is guaranteed to exist
	// once the client's handshake completes — otherwise an event published
	// between upgrade and subscribe would be missed.
	events, cancel, err := s.svc.Subscribe()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer cancel()

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case evt, ok := <-events:
			if !ok {
				return
			}
			if evt.ChatID != chatID {
				continue
			}
			if err := conn.WriteJSON(evt); err != nil {
				return
			}
		case <-ticker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// GenerateKeyPairResponse is returned by the key-generation endpoint.
type GenerateKeyPairResponse struct {
	PublicKey  []byte `json:"public_key"`
	PrivateKey []byte `json:"private_key"`
}

// HandleGenerateKeys is a convenience endpoint for local development that mints
// a fresh Ed25519 key pair. Production clients generate keys locally and never
// transmit private keys.
func (s *Server) HandleGenerateKeys(w http.ResponseWriter, _ *http.Request) {
	priv, pub, err := crypto.GenerateKeyPair()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, GenerateKeyPairResponse{PublicKey: pub, PrivateKey: priv})
}
