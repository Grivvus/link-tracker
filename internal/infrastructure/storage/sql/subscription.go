package sql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	gen "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/storage/sql/gen"
)

type SubscriptionRepositorySQL struct {
	queries *gen.Queries
	pool    *pgxpool.Pool
}

func NewSubscriptionRepository(pool *pgxpool.Pool) repository.SubscriptionRepository {
	return &SubscriptionRepositorySQL{
		queries: gen.New(pool),
		pool:    pool,
	}
}

func (repo *SubscriptionRepositorySQL) Subscribe(
	ctx context.Context, link domain.Link, chatID int64, tags []string,
) (domain.Subscription, error) {
}

func (repo *SubscriptionRepositorySQL) Unsubscribe(
	ctx context.Context, link domain.Link, chatID int64,
) (domain.Subscription, error) {
}

func (repo *SubscriptionRepositorySQL) GetLink(
	ctx context.Context, linkID int64,
) (domain.Link, error) {
	link, err := repo.queries.GetLinkByID(ctx, linkID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Link{}, fmt.Errorf(
				"%w: can't find link with thid id", repository.ErrEntityNotFound,
			)
		}
		return domain.Link{}, fmt.Errorf(
			"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
		)
	}
	domLink, err := domain.NewLink(link.Url)
	if err != nil {
		// err always should be nil, cause we load data from db,
		// so all data already was validated, when it was written to db
		return domain.Link{}, fmt.Errorf("unreachable, error always should be nil")
	}
	domLink.ID = link.ID
	domLink.LastUpdate = link.LastUpdate.Time
	return domLink, nil
}

func (repo *SubscriptionRepositorySQL) GetAllLinks(
	ctx context.Context,
) ([]domain.Link, error) {
	// сейчас - запрос просто тащит все записи из таблицы
	// а функция возвращает массив
	// в чистовой версии надо сделать, что бы запрос в бд брал курсор
	// а тут возвращался канал
	links, err := repo.queries.GetAllLinks(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf(
			"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
		)
	}
	domLinks := make([]domain.Link, len(links))
	for i, link := range links {
		domLink, err := domain.NewLink(link.Url)
		if err != nil {
			return nil, fmt.Errorf("unreachable, error always should be nil")
		}
		domLink.ID = link.ID
		domLink.LastUpdate = link.LastUpdate.Time
		domLinks[i] = domLink
	}
	return domLinks, nil
}

func (repo *SubscriptionRepositorySQL) UpdateLink(
	ctx context.Context, link domain.Link,
) (domain.Link, error) {
	// should never happen
	if link.ID == 0 {
		return domain.Link{}, fmt.Errorf(
			"%w: expect link with valid ID", repository.ErrEntityNotFound,
		)
	}
	updatedLink, err := repo.queries.UpdateLink(ctx, gen.UpdateLinkParams{
		ID:         link.ID,
		LastUpdate: pgtype.Timestamptz{Time: link.LastUpdate, Valid: true},
	})
	if err != nil {
		return domain.Link{}, fmt.Errorf(
			"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
		)
	}
	domLink, err := domain.NewLink(updatedLink.Url)
	if err != nil {
		return domain.Link{}, fmt.Errorf("unreachable, error always should be nil")
	}
	domLink.ID = link.ID
	domLink.LastUpdate = updatedLink.LastUpdate.Time
	return domLink, nil
}

func (repo *SubscriptionRepositorySQL) FindLinkByURL(
	ctx context.Context, url *url.URL,
) (domain.Link, error) {
	link, err := repo.queries.FindLinkByURL(ctx, url.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Link{}, fmt.Errorf(
				"%w: can't find link with this url",
				repository.ErrEntityNotFound,
			)
		}
		return domain.Link{}, fmt.Errorf(
			"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
		)
	}
	domLink, err := domain.NewLink(link.Url)
	if err != nil {
		return domain.Link{}, fmt.Errorf("unreachable, error always should be nil")
	}
	domLink.ID = link.ID
	domLink.LastUpdate = link.LastUpdate.Time
	return domLink, nil
}

func (repo *SubscriptionRepositorySQL) AllChatSubscriptions(
	ctx context.Context, chatID int64,
) ([]domain.Subscription, error) {
	tx, err := repo.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf(
			"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
		)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := repo.queries.WithTx(tx)
	subs, err := qtx.FindSubscriptionsByChat(
		ctx, pgtype.Int8{Int64: chatID, Valid: true},
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: msg - %v", repository.ErrUnknownDBError, err.Error())
	}
	domSubs := make([]domain.Subscription, len(subs))
	for i, sub := range subs {
		tags, err := qtx.FindTagBySubscription(ctx, sub.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf(
				"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
			)
		}
		tagsStr := make([]string, len(tags))
		for i, tag := range tags {
			tagsStr[i] = tag.Name.String
		}
		domSubs[i] = domain.NewSubscription(
			sub.ID, sub.LinkID.Int64, sub.ChatID.Int64, tagsStr,
		)
	}
	return domSubs, tx.Commit(ctx)
}

func (repo *SubscriptionRepositorySQL) AllLinkSubscriptions(
	ctx context.Context, link domain.Link,
) ([]domain.Subscription, error) {
	if link.ID == 0 {
		var err error
		link, err = repo.FindLinkByURL(ctx, link.URL)
		if err != nil {
			return nil, err
		}
	}
	tx, err := repo.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf(
			"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
		)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := repo.queries.WithTx(tx)
	subs, err := qtx.FindSubscriptionsByLink(
		ctx, pgtype.Int8{Int64: link.ID, Valid: true},
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: msg - %v", repository.ErrUnknownDBError, err.Error())
	}
	domSubs := make([]domain.Subscription, len(subs))
	for i, sub := range subs {
		tags, err := qtx.FindTagBySubscription(ctx, sub.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf(
				"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
			)
		}
		tagsStr := make([]string, len(tags))
		for i, tag := range tags {
			tagsStr[i] = tag.Name.String
		}
		domSubs[i] = domain.NewSubscription(
			sub.ID, sub.LinkID.Int64, sub.ChatID.Int64, tagsStr,
		)
	}
	return domSubs, tx.Commit(ctx)
}
