package repository

import "context"

type LinksMetricsRepository interface {
	TrackedLinksCount(ctx context.Context) (gh_count, st_count uint, err error)
}
