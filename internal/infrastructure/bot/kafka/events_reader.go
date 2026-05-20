package kafka

import (
	kafkaCommon "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/kafka"
)

type EventsReader struct {
	kafkaCommon.EventsReader
}

func NewEventsReader(
	reader kafkaCommon.EventsReader,
) *EventsReader {
	return &EventsReader{reader}
}
