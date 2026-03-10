// heartbeat/heartbeat.go
// Tipos compartidos para el protocolo de heartbeat entre agente y operador.
package heartbeat

import "time"

// Payload es el cuerpo JSON que el agente envía al operador.
type Payload struct {
	NodeName  string    `json:"nodeName"`
	Timestamp time.Time `json:"timestamp"`
	CPU       string    `json:"cpu"`
	Memory    string    `json:"memory"`
}
