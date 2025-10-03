package contracts

import "time"

type WSError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type WSMessage[T any] struct {
	Type          string    `json:"type"`                    // ví dụ: "onCollectionStatus"
	Version       int       `json:"version"`                 // schema version
	CorrelationID string    `json:"correlationId,omitempty"` // intentId/requestId
	TraceID       string    `json:"traceId,omitempty"`
	EmittedAt     time.Time `json:"emittedAt"` // server time (UTC)
	Data          T         `json:"data"`
	Error         *WSError  `json:"error,omitempty"`
}
