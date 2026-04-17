package infrastructure

import (
	"fmt"
)

func KafkaLoggerFn(sloggerFn func(string, ...any)) func(string, ...any) {
	return func(format string, v ...any) {
		sloggerFn(fmt.Sprintf(format, v...))
	}
}
