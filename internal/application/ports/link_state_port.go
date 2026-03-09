package ports

import (
	"errors"
	"net/url"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

var ErrBadResponse = errors.New("got non 2XX response code")
var ErrBadURL = errors.New("unknown hostname, expected github.com or stackoverflow.com")

type LinkStatePort interface {
	Fetch(url *url.URL) (*domain.Snapshot, error)
}
