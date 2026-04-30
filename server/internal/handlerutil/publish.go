package handlerutil

import (
	"github.com/agentra-ai/agentra/server/internal/events"
)

type EventPublisher interface {
	Publish(events.Event)
}

// Publish sends a domain event through the event bus.
func Publish(publisher EventPublisher, eventType, workspaceID, actorType, actorID string, payload any) {
	publisher.Publish(events.Event{
		Type:        eventType,
		WorkspaceID: workspaceID,
		ActorType:   actorType,
		ActorID:     actorID,
		Payload:     payload,
	})
}