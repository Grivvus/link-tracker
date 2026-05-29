package kafka

import commonKafka "link-tracker/internal/infrastructure/kafka"

type DeferredRawEventsReader struct {
	*commonKafka.DeferredEventsReader
}

func NewDeferredRawEventsReader(
	reader *commonKafka.DeferredEventsReader,
) *DeferredRawEventsReader {
	return &DeferredRawEventsReader{DeferredEventsReader: reader}
}
