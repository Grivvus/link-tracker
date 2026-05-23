package kafka

import commonKafka "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/kafka"

type DeferredRawEventsReader struct {
	*commonKafka.DeferredEventsReader
}

func NewDeferredRawEventsReader(
	reader *commonKafka.DeferredEventsReader,
) *DeferredRawEventsReader {
	return &DeferredRawEventsReader{DeferredEventsReader: reader}
}
