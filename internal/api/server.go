// Package api exposes the messenger over HTTP/JSON.
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/chatlog"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/crypto"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/models"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/service"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/storage"
)

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
	mux.HandleFunc("POST /chats/{chatID}/photos", s.handleSendPhoto)
	mux.HandleFunc("GET /chats/{chatID}/verify", s.handleVerify)
	mux.HandleFunc("GET /chats/{chatID}/sync", s.handleSync)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type sendRequest struct {
	SenderID   string `json:"sender_id"`
	PublicKey  []byte `json:"public_key"`
	PrivateKey []byte `json:"private_key"`
	Text       string `json:"text"`
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	chatID := r.PathValue("chatID")
	var req sendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.SenderID == "" || len(req.PublicKey) == 0 || len(req.PrivateKey) == 0 {
		writeError(w, http.StatusBadRequest, "sender_id, public_key and private_key are required")
		return
	}

	entry, err := s.svc.SendText(chatID, req.SenderID, req.PublicKey, req.PrivateKey, req.Text)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, entry)
}

type sendPhotoRequest struct {
	SenderID    string `json:"sender_id"`
	PublicKey   []byte `json:"public_key"`
	PrivateKey  []byte `json:"private_key"`
	ContentKey  []byte `json:"content_key"`
	Photo       []byte `json:"photo"`
	ContentType string `json:"content_type"`
	Filename    string `json:"filename"`
}

func (s *Server) handleSendPhoto(w http.ResponseWriter, r *http.Request) {
	chatID := r.PathValue("chatID")
	var req sendPhotoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.SenderID == "" || len(req.PublicKey) == 0 || len(req.PrivateKey) == 0 {
		writeError(w, http.StatusBadRequest, "sender_id, public_key and private_key are required")
		return
	}
	if len(req.ContentKey) == 0 || len(req.Photo) == 0 {
		writeError(w, http.StatusBadRequest, "content_key and photo are required")
		return
	}

	sender := service.Sender{ID: req.SenderID, PublicKey: req.PublicKey, PrivateKey: req.PrivateKey}
	entry, err := s.svc.SendPhoto(chatID, sender, req.ContentKey, req.Photo, req.ContentType, req.Filename)
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
