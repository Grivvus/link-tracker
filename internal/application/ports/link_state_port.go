package ports

import (
	"context"
	"errors"

	"link-tracker/internal/application/model"
	"link-tracker/internal/domain"
)

var ErrBadResponse = errors.New("got non 2XX response code")
var ErrBadURL = errors.New("unknown hostname, expected github.com or stackoverflow.com")

type LinkStatePort interface {
	Fetch(ctx context.Context, link domain.Link) ([]model.LinkState, error)
}
