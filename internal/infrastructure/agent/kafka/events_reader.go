package kafka

import (
	kafkaCommon "link-tracker/internal/infrastructure/kafka"
)

type RawEventsReader struct {
	kafkaCommon.EventsReader
}

func NewRawEventsReader(
	reader kafkaCommon.EventsReader,
) *RawEventsReader {
	return &RawEventsReader{reader}
}
