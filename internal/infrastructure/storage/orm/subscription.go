package orm

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/doug-martin/goqu/v9"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

const pageSize = 50

type SubscriptionRepositoryORM struct {
	pool *pgxpool.Pool
}

func NewSubscriptionRepository(pool *pgxpool.Pool) repository.SubscriptionRepository {
	return &SubscriptionRepositoryORM{
		pool: pool,
	}
}

func (repo *SubscriptionRepositoryORM) Subscribe(
	ctx context.Context, link domain.Link, chatID int64, tags []string,
) (domain.Subscription, error) {
	return withTx(ctx, repo.pool, func(tx pgx.Tx) (domain.Subscription, error) {
		isReg, err := repo.isChatRegistered(ctx, tx, chatID)
		if err != nil {
			return domain.Subscription{}, err
		}
		if !isReg {
			return domain.Subscription{}, fmt.Errorf(
				"%w: no chat with thid id", repository.ErrEntityNotFound,
			)
		}

		dbLink, err := repo.getOrCreateLink(ctx, tx, link)
		if err != nil {
			return domain.Subscription{}, err
		}

		_, err = repo.findSubscriptionByLinkAndChat(ctx, tx, dbLink.ID, chatID)
		if err == nil {
			return domain.Subscription{}, repository.ErrEntityAlreadyExists
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return domain.Subscription{}, wrapUnknownDBError(err)
		}

		dbSub, err := queryOne(ctx, tx, dialect.
			Insert("subscription").
			Cols("link_id", "chat_id").
			Vals(goqu.Vals{dbLink.ID, chatID}).
			Returning("id", "link_id", "chat_id").
			Prepared(true), pgx.RowToStructByName[Subscription])
		if err != nil {
			return domain.Subscription{}, wrapUnknownDBError(err)
		}

		for _, tagName := range tags {
			dbTag, err := repo.getOrCreateTag(ctx, tx, tagName)
			if err != nil {
				return domain.Subscription{}, err
			}

			err = execDataset(ctx, tx, dialect.
				Insert("subscription_tag").
				Cols("subscription_id", "tag_id").
				Vals(goqu.Vals{dbSub.ID, dbTag.ID}).
				Prepared(true))
			if err != nil {
				return domain.Subscription{}, wrapUnknownDBError(err)
			}
		}

		return toDomainSubscription(dbSub, tags), nil
	})
}

func (repo *SubscriptionRepositoryORM) Unsubscribe(
	ctx context.Context, link domain.Link, chatID int64,
) (domain.Subscription, error) {
	return withTx(ctx, repo.pool, func(tx pgx.Tx) (domain.Subscription, error) {
		isReg, err := repo.isChatRegistered(ctx, tx, chatID)
		if err != nil {
			return domain.Subscription{}, err
		}
		if !isReg {
			return domain.Subscription{}, fmt.Errorf(
				"%w: no chat with thid id", repository.ErrEntityNotFound,
			)
		}

		linkURL, err := requireURL(link.URL)
		if err != nil {
			return domain.Subscription{}, err
		}

		dbLink, err := repo.findLinkByURLRecord(ctx, tx, linkURL)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.Subscription{}, fmt.Errorf(
					"%w: can't find this url", repository.ErrEntityNotFound,
				)
			}
			return domain.Subscription{}, wrapUnknownDBError(err)
		}

		dbSub, err := repo.findSubscriptionByLinkAndChat(ctx, tx, dbLink.ID, chatID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.Subscription{}, fmt.Errorf(
					"%w: subscription doesn't exist", repository.ErrEntityNotFound,
				)
			}
			return domain.Subscription{}, wrapUnknownDBError(err)
		}

		dbTags, err := repo.findTagsBySubscription(ctx, tx, dbSub.ID)
		if err != nil {
			return domain.Subscription{}, err
		}

		_, err = queryOne(ctx, tx, dialect.
			Delete("subscription").
			Where(goqu.Ex{"id": dbSub.ID}).
			Returning("id", "link_id", "chat_id").
			Prepared(true), pgx.RowToStructByName[Subscription])
		if err != nil {
			return domain.Subscription{}, wrapUnknownDBError(err)
		}

		tags := make([]string, len(dbTags))
		for i, dbTag := range dbTags {
			tags[i] = dbTag.Name
		}

		return toDomainSubscription(dbSub, tags), nil
	})
}

func (repo *SubscriptionRepositoryORM) GetLink(
	ctx context.Context, linkID int64,
) (domain.Link, error) {
	dbLink, err := queryOne(ctx, repo.pool, dialect.
		Select("id", "url", "last_update").
		From("link").
		Where(goqu.Ex{"id": linkID}).
		Prepared(true), pgx.RowToStructByName[Link])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Link{}, fmt.Errorf(
				"%w: can't find link with thid id", repository.ErrEntityNotFound,
			)
		}
		return domain.Link{}, wrapUnknownDBError(err)
	}

	return toDomainLink(dbLink)
}

func (repo *SubscriptionRepositoryORM) GetAllLinks(
	ctx context.Context,
) (<-chan domain.Link, <-chan error) {
	ch := make(chan domain.Link)
	errCh := make(chan error, 1)
	go func() {
		defer close(ch)
		defer close(errCh)
		var lastLinkID int64
		for {
			ds := dialect.
				Select("id", "url", "last_update").
				From("link").
				Where(goqu.Ex{"id": goqu.Op{"gt": lastLinkID}}).
				Limit(pageSize).
				Order(goqu.C("id").Asc()).
				Prepared(true)

			dbLinks, err := queryMany(ctx, repo.pool, ds, pgx.RowToStructByName[Link])
			if err != nil {
				errCh <- wrapUnknownDBError(err)
				return
			}
			for _, dbLink := range dbLinks {
				domLink, err := toDomainLink(dbLink)
				if err != nil {
					errCh <- err
					return
				}
				ch <- domLink
				lastLinkID = dbLink.ID
			}
			if len(dbLinks) < pageSize {
				return
			}
		}
	}()
	return ch, errCh

}

func (repo *SubscriptionRepositoryORM) UpdateLink(
	ctx context.Context, link domain.Link,
) (domain.Link, error) {
	if link.ID == 0 {
		return domain.Link{}, fmt.Errorf(
			"%w: expect link with valid ID", repository.ErrEntityNotFound,
		)
	}

	dbLink, err := queryOne(ctx, repo.pool, dialect.
		Update("link").
		Set(goqu.Record{
			"last_update": pgtype.Timestamptz{Time: link.LastUpdate, Valid: true},
		}).
		Where(goqu.Ex{"id": link.ID}).
		Returning("id", "url", "last_update").
		Prepared(true), pgx.RowToStructByName[Link])
	if err != nil {
		return domain.Link{}, wrapUnknownDBError(err)
	}

	return toDomainLink(dbLink)
}

func (repo *SubscriptionRepositoryORM) FindLinkByURL(
	ctx context.Context, rawURL *url.URL,
) (domain.Link, error) {
	urlStr, err := requireURL(rawURL)
	if err != nil {
		return domain.Link{}, err
	}

	dbLink, err := repo.findLinkByURLRecord(ctx, repo.pool, urlStr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Link{}, fmt.Errorf(
				"%w: can't find link with this url",
				repository.ErrEntityNotFound,
			)
		}
		return domain.Link{}, wrapUnknownDBError(err)
	}

	return toDomainLink(dbLink)
}

func (repo *SubscriptionRepositoryORM) AllChatSubscriptions(
	ctx context.Context, chatID int64,
) ([]domain.Subscription, error) {
	return withTx(ctx, repo.pool, func(tx pgx.Tx) ([]domain.Subscription, error) {
		isReg, err := repo.isChatRegistered(ctx, tx, chatID)
		if err != nil {
			return nil, err
		}
		if !isReg {
			return nil, fmt.Errorf(
				"%w: no chat with thid id", repository.ErrEntityNotFound,
			)
		}

		return repo.loadSubscriptionsWithTags(ctx, tx, dialect.
			Select("id", "link_id", "chat_id").
			From("subscription").
			Where(goqu.Ex{"chat_id": chatID}).
			Prepared(true))
	})
}

func (repo *SubscriptionRepositoryORM) AllLinkSubscriptions(
	ctx context.Context, link domain.Link,
) ([]domain.Subscription, error) {
	linkID := link.ID
	if linkID == 0 {
		foundLink, err := repo.FindLinkByURL(ctx, link.URL)
		if err != nil {
			return nil, err
		}
		linkID = foundLink.ID
	}

	return withTx(ctx, repo.pool, func(tx pgx.Tx) ([]domain.Subscription, error) {
		return repo.loadSubscriptionsWithTags(ctx, tx, dialect.
			Select("id", "link_id", "chat_id").
			From("subscription").
			Where(goqu.Ex{"link_id": linkID}).
			Prepared(true))
	})
}

func (repo *SubscriptionRepositoryORM) isChatRegistered(
	ctx context.Context, db dbRunner, chatID int64,
) (bool, error) {
	_, err := queryOne(ctx, db, dialect.
		Select("id").
		From("chat").
		Where(goqu.Ex{"id": chatID}).
		Prepared(true), pgx.RowTo[int64])
	if err == nil {
		return true, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}

	return false, wrapUnknownDBError(err)
}

func (repo *SubscriptionRepositoryORM) getOrCreateLink(
	ctx context.Context, db dbRunner, link domain.Link,
) (Link, error) {
	linkURL, err := requireURL(link.URL)
	if err != nil {
		return Link{}, err
	}

	dbLink, err := repo.findLinkByURLRecord(ctx, db, linkURL)
	if err == nil {
		return dbLink, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Link{}, wrapUnknownDBError(err)
	}

	dbLink, err = queryOne(ctx, db, dialect.
		Insert("link").
		Cols("url").
		Vals(goqu.Vals{linkURL}).
		Returning("id", "url", "last_update").
		Prepared(true), pgx.RowToStructByName[Link])
	if err != nil {
		return Link{}, wrapUnknownDBError(err)
	}

	return dbLink, nil
}

func (repo *SubscriptionRepositoryORM) findLinkByURLRecord(
	ctx context.Context, db dbRunner, linkURL string,
) (Link, error) {
	return queryOne(ctx, db, dialect.
		Select("id", "url", "last_update").
		From("link").
		Where(goqu.Ex{"url": linkURL}).
		Prepared(true), pgx.RowToStructByName[Link])
}

func (repo *SubscriptionRepositoryORM) findSubscriptionByLinkAndChat(
	ctx context.Context, db dbRunner, linkID, chatID int64,
) (Subscription, error) {
	return queryOne(ctx, db, dialect.
		Select("id", "link_id", "chat_id").
		From("subscription").
		Where(goqu.Ex{
			"chat_id": chatID,
			"link_id": linkID,
		}).
		Prepared(true), pgx.RowToStructByName[Subscription])
}

func (repo *SubscriptionRepositoryORM) getOrCreateTag(
	ctx context.Context, db dbRunner, tagName string,
) (Tag, error) {
	dbTag, err := queryOne(ctx, db, dialect.
		Select("id", "name").
		From("tag").
		Where(goqu.Ex{"name": tagName}).
		Prepared(true), pgx.RowToStructByName[Tag])
	if err == nil {
		return dbTag, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Tag{}, wrapUnknownDBError(err)
	}

	dbTag, err = queryOne(ctx, db, dialect.
		Insert("tag").
		Cols("name").
		Vals(goqu.Vals{tagName}).
		Returning("id", "name").
		Prepared(true), pgx.RowToStructByName[Tag])
	if err != nil {
		return Tag{}, wrapUnknownDBError(err)
	}

	return dbTag, nil
}

func (repo *SubscriptionRepositoryORM) findTagsBySubscription(
	ctx context.Context, db dbRunner, subscriptionID int64,
) ([]Tag, error) {
	dbTags, err := queryMany(ctx, db, dialect.
		Select(
			goqu.I("tag.id").As("id"),
			goqu.I("tag.name").As("name"),
		).
		From("tag").
		InnerJoin(
			goqu.T("subscription_tag"),
			goqu.On(goqu.Ex{
				"tag.id":                           goqu.I("subscription_tag.tag_id"),
				"subscription_tag.subscription_id": subscriptionID,
			}),
		).
		Prepared(true), pgx.RowToStructByName[Tag])
	if err != nil {
		return nil, wrapUnknownDBError(err)
	}

	return dbTags, nil
}

func (repo *SubscriptionRepositoryORM) loadSubscriptionsWithTags(
	ctx context.Context, db dbRunner, ds *goqu.SelectDataset,
) ([]domain.Subscription, error) {
	dbSubs, err := queryMany(ctx, db, ds, pgx.RowToStructByName[Subscription])
	if err != nil {
		return nil, wrapUnknownDBError(err)
	}

	domSubs := make([]domain.Subscription, len(dbSubs))
	for i, dbSub := range dbSubs {
		dbTags, err := repo.findTagsBySubscription(ctx, db, dbSub.ID)
		if err != nil {
			return nil, err
		}

		tags := make([]string, len(dbTags))
		for j, dbTag := range dbTags {
			tags[j] = dbTag.Name
		}

		domSubs[i] = toDomainSubscription(dbSub, tags)
	}

	return domSubs, nil
}
