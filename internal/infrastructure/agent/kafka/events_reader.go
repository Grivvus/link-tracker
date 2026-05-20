package kafka

import (
	kafkaCommon "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/kafka"
)

type RawEventsReader struct {
	kafkaCommon.EventsReader
}

func NewRawEventsReader(
	reader kafkaCommon.EventsReader,
) *RawEventsReader {
	return &RawEventsReader{reader}
}
