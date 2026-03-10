// internal/heartbeatserver/server.go
// HeartbeatServer expone un endpoint HTTP POST /heartbeat que los agentes
// llaman periódicamente. Registra cada payload en el HeartbeatStore.
package heartbeatserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"

	"github.com/jaiderssjgod/edge-operator/heartbeat"
	"github.com/jaiderssjgod/edge-operator/internal/heartbeatstore"
)

// Server es el servidor HTTP que recibe heartbeats.
type Server struct {
	store  *heartbeatstore.Store
	log    logr.Logger
	addr   string
	server *http.Server
}

// New crea un Server que escucha en addr y almacena en store.
func New(addr string, store *heartbeatstore.Store, log logr.Logger) *Server {
	s := &Server{
		store: store,
		log:   log,
		addr:  addr,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/heartbeat", s.handleHeartbeat)

	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return s
}

// Start arranca el servidor HTTP en una goroutine.
// Llama a s.server.ListenAndServe, que bloquea; se debe invocar con `go`.
func (s *Server) Start() {
	s.log.Info("Starting heartbeat HTTP server", "addr", s.addr)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.log.Error(err, "Heartbeat server stopped unexpectedly")
	}
}

// handleHeartbeat procesa POST /heartbeat.
func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload heartbeat.Payload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		s.log.Error(err, "Failed to decode heartbeat payload")
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if payload.NodeName == "" {
		http.Error(w, "nodeName is required", http.StatusBadRequest)
		return
	}

	s.store.Record(payload)
	s.log.V(1).Info("Heartbeat received", "node", payload.NodeName, "ts", payload.Timestamp)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}
