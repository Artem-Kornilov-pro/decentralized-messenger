// Package api exposes the messenger over HTTP/JSON.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/crypto"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/service"
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
	mux.HandleFunc("POST /chats/{chatID}/messages", s.handleSend)
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
