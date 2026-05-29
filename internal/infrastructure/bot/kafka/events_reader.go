package kafka

import (
	kafkaCommon "link-tracker/internal/infrastructure/kafka"
)

type EventsReader struct {
	kafkaCommon.EventsReader
}

func NewEventsReader(
	reader kafkaCommon.EventsReader,
) *EventsReader {
	return &EventsReader{reader}
}
