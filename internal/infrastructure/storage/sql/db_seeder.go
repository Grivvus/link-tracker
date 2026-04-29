package sql

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/jackc/pgx/v5/pgxpool"
	db "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/storage/sql/gen"
)

type DBSeeder struct {
	q    *db.Queries
	pool *pgxpool.Pool
}

func NewDBSeeder(pool *pgxpool.Pool) *DBSeeder {
	return &DBSeeder{
		pool: pool,
		q:    db.New(pool),
	}
}

func (s *DBSeeder) SeedLinkAndUsers(ctx context.Context, nlinks, nusers uint) (int64, error) {
	return withTx(ctx, s.pool, s.q, func(q *db.Queries) (int64, error) {
		var initialID int64 = 10e12
		for i := range nlinks {
			isGH := rand.Intn(2)
			var url string
			if isGH == 1 {
				url = makeGithubURL(i)
			} else {
				url = makeStackoverflowURL(i)
			}
			link, err := q.AddLink(ctx, url)
			if err != nil {
				return 0, err
			}
			initialID = min(initialID, link.ID)
		}
		for i := range nusers {
			err := q.RegisterChat(ctx, int64(i))
			if err != nil {
				return 0, err
			}
		}
		return initialID, nil
	})
}

func (s *DBSeeder) SeedSubscriptions(ctx context.Context, linksStartID int64, nlinks, nusers uint) error {
	_, err := withTx(ctx, s.pool, s.q, func(q *db.Queries) (int, error) {
		for i := range nusers {
			for range 100 {
				_, err := q.Subscribe(ctx, db.SubscribeParams{
					LinkID: rand.Int63n(int64(nlinks)) + linksStartID,
					ChatID: int64(i),
				})
				if err != nil {
					continue
				}
			}
		}
		return 0, nil
	})
	return err
}

func (s *DBSeeder) ClearDB(ctx context.Context) error {
	query := `DELETE FROM subscription; DELETE FROM link; DELETE FROM chat;`
	_, err := s.pool.Exec(ctx, query)
	return err
}

func makeGithubURL(n uint) string {
	return fmt.Sprintf("https://github.com/user-%d/repo-%d", n, n)
}

func makeStackoverflowURL(n uint) string {
	return fmt.Sprintf("https://stackoverflow.com/questions/%d", n)
}
