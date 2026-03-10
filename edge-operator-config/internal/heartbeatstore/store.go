// internal/heartbeatstore/store.go
// HeartbeatStore mantiene el último heartbeat recibido por cada nodo.
// Es seguro para uso concurrente (múltiples goroutines leen/escriben).
package heartbeatstore

import (
	"sync"
	"time"

	"github.com/jaiderssjgod/edge-operator/heartbeat"
)

// NodeState resume el estado derivado de los heartbeats almacenados.
type NodeState struct {
	LastHeartbeat time.Time
	CPU           string
	Memory        string
	// Offline es true cuando no se ha recibido heartbeat en los últimos timeoutDuration.
	Offline bool
}

// Store guarda el último heartbeat de cada nodo y expone métodos para
// consultarlos y determinar si un nodo está offline.
type Store struct {
	mu              sync.RWMutex
	records         map[string]heartbeat.Payload
	timeoutDuration time.Duration
}

// New crea un Store con el timeout de desconexión indicado.
func New(timeout time.Duration) *Store {
	return &Store{
		records:         make(map[string]heartbeat.Payload),
		timeoutDuration: timeout,
	}
}

// Record almacena (o sobreescribe) el heartbeat más reciente de un nodo.
func (s *Store) Record(p heartbeat.Payload) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[p.NodeName] = p
}

// GetNodeState devuelve el estado actual de un nodo dado su nombre.
// Si el nodo no tiene ningún heartbeat registrado, Offline=true y LastHeartbeat es zero.
func (s *Store) GetNodeState(nodeName string) NodeState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, exists := s.records[nodeName]
	if !exists {
		return NodeState{Offline: true}
	}

	offline := time.Since(p.Timestamp) > s.timeoutDuration
	return NodeState{
		LastHeartbeat: p.Timestamp,
		CPU:           p.CPU,
		Memory:        p.Memory,
		Offline:       offline,
	}
}

// Snapshot devuelve una copia del mapa de payloads para que el reconciler
// pueda iterar sin mantener el lock.
func (s *Store) Snapshot() map[string]heartbeat.Payload {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]heartbeat.Payload, len(s.records))
	for k, v := range s.records {
		out[k] = v
	}
	return out
}
