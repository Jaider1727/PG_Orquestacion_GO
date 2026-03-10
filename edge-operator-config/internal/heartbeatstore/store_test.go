// internal/heartbeatstore/store_test.go
package heartbeatstore_test

import (
	"testing"
	"time"

	"github.com/jaiderssjgod/edge-operator/heartbeat"
	"github.com/jaiderssjgod/edge-operator/internal/heartbeatstore"
)

func TestGetNodeState_NoHeartbeat_IsOffline(t *testing.T) {
	store := heartbeatstore.New(30 * time.Second)
	state := store.GetNodeState("node-edge-1")
	if !state.Offline {
		t.Error("expected node without heartbeat to be offline")
	}
}

func TestGetNodeState_FreshHeartbeat_IsOnline(t *testing.T) {
	store := heartbeatstore.New(30 * time.Second)
	store.Record(heartbeat.Payload{
		NodeName:  "node-edge-1",
		Timestamp: time.Now(),
		CPU:       "10.00%",
		Memory:    "45.00%",
	})

	state := store.GetNodeState("node-edge-1")
	if state.Offline {
		t.Error("expected node with recent heartbeat to be online")
	}
	if state.CPU != "10.00%" {
		t.Errorf("unexpected CPU value: %s", state.CPU)
	}
}

func TestGetNodeState_StaleHeartbeat_IsOffline(t *testing.T) {
	store := heartbeatstore.New(30 * time.Second)
	store.Record(heartbeat.Payload{
		NodeName:  "node-edge-1",
		Timestamp: time.Now().Add(-60 * time.Second), // 60 s atrás → supera timeout de 30 s
		CPU:       "5.00%",
		Memory:    "30.00%",
	})

	state := store.GetNodeState("node-edge-1")
	if !state.Offline {
		t.Error("expected node with stale heartbeat to be offline")
	}
}

func TestRecord_OverwritesPreviousHeartbeat(t *testing.T) {
	store := heartbeatstore.New(30 * time.Second)

	// Primer heartbeat viejo
	store.Record(heartbeat.Payload{
		NodeName:  "node-edge-2",
		Timestamp: time.Now().Add(-60 * time.Second),
		CPU:       "1.00%",
	})

	// Segundo heartbeat fresco
	store.Record(heartbeat.Payload{
		NodeName:  "node-edge-2",
		Timestamp: time.Now(),
		CPU:       "20.00%",
	})

	state := store.GetNodeState("node-edge-2")
	if state.Offline {
		t.Error("expected node to be online after fresh heartbeat")
	}
	if state.CPU != "20.00%" {
		t.Errorf("expected updated CPU value, got: %s", state.CPU)
	}
}

func TestSnapshot_ReturnsCopy(t *testing.T) {
	store := heartbeatstore.New(30 * time.Second)
	store.Record(heartbeat.Payload{NodeName: "node-a", Timestamp: time.Now()})
	store.Record(heartbeat.Payload{NodeName: "node-b", Timestamp: time.Now()})

	snap := store.Snapshot()
	if len(snap) != 2 {
		t.Errorf("expected 2 entries in snapshot, got %d", len(snap))
	}

	// Modificar el snapshot no debe afectar al store
	delete(snap, "node-a")
	snap2 := store.Snapshot()
	if len(snap2) != 2 {
		t.Error("modifying snapshot should not affect the store")
	}
}
