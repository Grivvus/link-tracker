package sql

import (
	"context"
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
	isReg, err := repo.isChatRegistered(ctx, chatID)
	if err != nil {
		return domain.Subscription{}, err
	}
	if !isReg {
		return domain.Subscription{}, fmt.Errorf(
			"%w: no chat with thid id", repository.ErrEntityNotFound,
		)
	}
	link, err = repo.getOrCreateLink(ctx, link)
	if err != nil {
		return domain.Subscription{}, err
	}

	return withTx(ctx, repo.pool, repo.queries, func(queries *gen.Queries) (domain.Subscription, error) {
		_, err = queries.FindSubscriptionByLinkAndChat(
			ctx, gen.FindSubscriptionByLinkAndChatParams{
				ChatID: chatID,
				LinkID: link.ID,
			},
		)
		if err == nil {
			return domain.Subscription{}, repository.ErrEntityAlreadyExists
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return domain.Subscription{}, fmt.Errorf(
				"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
			)
		}
		sub, err := queries.Subscribe(ctx, gen.SubscribeParams{
			LinkID: link.ID,
			ChatID: chatID,
		})
		if err != nil {
			return domain.Subscription{}, fmt.Errorf(
				"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
			)
		}
		for _, tag := range tags {
			dbTag, err := repo.getOrCreateTag(ctx, tag)
			if err != nil {
				return domain.Subscription{}, err
			}
			err = queries.AddSubscriptionTag(ctx, gen.AddSubscriptionTagParams{
				SubscriptionID: sub.ID,
				TagID:          dbTag.ID,
			})
			if err != nil {
				return domain.Subscription{}, fmt.Errorf(
					"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
				)
			}
		}
		return domain.Subscription{
			ID:     sub.ID,
			ChatID: chatID,
			LinkID: link.ID,
			Tags:   tags,
		}, nil
	})
}

func (repo *SubscriptionRepositorySQL) Unsubscribe(
	ctx context.Context, link domain.Link, chatID int64,
) (domain.Subscription, error) {
	isReg, err := repo.isChatRegistered(ctx, chatID)
	if err != nil {
		return domain.Subscription{}, err
	}
	if !isReg {
		return domain.Subscription{}, fmt.Errorf(
			"%w: no chat with thid id", repository.ErrEntityNotFound,
		)
	}
	return withTx(ctx, repo.pool, repo.queries, func(queries *gen.Queries) (domain.Subscription, error) {
		dbLink, err := queries.FindLinkByURL(ctx, link.URL.String())
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.Subscription{}, fmt.Errorf(
					"%w: can't find this url", repository.ErrEntityNotFound,
				)
			}
			return domain.Subscription{}, fmt.Errorf(
				"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
			)
		}
		link.ID = dbLink.ID
		sub, err := queries.FindSubscriptionByLinkAndChat(
			ctx, gen.FindSubscriptionByLinkAndChatParams{
				ChatID: chatID,
				LinkID: link.ID,
			})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.Subscription{}, fmt.Errorf(
					"%w: subscription doesn't exist", repository.ErrEntityNotFound,
				)
			}
			return domain.Subscription{}, fmt.Errorf(
				"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
			)
		}
		dbTags, err := queries.FindTagBySubscription(ctx, sub.ID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return domain.Subscription{}, fmt.Errorf(
				"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
			)
		}
		tags := make([]string, len(dbTags))
		for i, dbTag := range dbTags {
			tags[i] = dbTag.Name
		}
		_, err = queries.Unsubscribe(ctx, sub.ID)
		if err != nil {
			return domain.Subscription{}, fmt.Errorf(
				"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
			)
		}

		return domain.Subscription{
			ID:     sub.ID,
			ChatID: chatID,
			LinkID: link.ID,
			Tags:   tags,
		}, nil
	})
}

func (repo *SubscriptionRepositorySQL) GetLink(
	ctx context.Context, linkID int64,
) (domain.Link, error) {
	link, err := repo.queries.GetLinkByID(ctx, linkID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Link{}, fmt.Errorf(
				"%w: can't find link with thid id", repository.ErrEntityNotFound,
			)
		}
		return domain.Link{}, fmt.Errorf(
			"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
		)
	}
	return toDomainLink(link)
}

func (repo *SubscriptionRepositorySQL) GetAllLinks(
	ctx context.Context,
) ([]domain.Link, error) {
	// сейчас - запрос просто тащит все записи из таблицы
	// а функция возвращает массив
	// в чистовой версии надо сделать, что бы запрос в бд брал курсор
	// а тут возвращался канал
	links, err := repo.queries.GetAllLinks(ctx)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf(
			"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
		)
	}
	domLinks := make([]domain.Link, len(links))
	for i, link := range links {
		domLink, err := toDomainLink(link)
		if err != nil {
			return nil, err
		}
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
	return toDomainLink(updatedLink)
}

func (repo *SubscriptionRepositorySQL) FindLinkByURL(
	ctx context.Context, url *url.URL,
) (domain.Link, error) {
	link, err := repo.queries.FindLinkByURL(ctx, url.String())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Link{}, fmt.Errorf(
				"%w: can't find link with this url",
				repository.ErrEntityNotFound,
			)
		}
		return domain.Link{}, fmt.Errorf(
			"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
		)
	}
	return toDomainLink(link)
}

func (repo *SubscriptionRepositorySQL) AllChatSubscriptions(
	ctx context.Context, chatID int64,
) ([]domain.Subscription, error) {
	isReg, err := repo.isChatRegistered(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if !isReg {
		return nil, fmt.Errorf(
			"%w: no chat with thid id", repository.ErrEntityNotFound,
		)
	}
	return withTx(ctx, repo.pool, repo.queries, func(queries *gen.Queries) ([]domain.Subscription, error) {
		subs, err := queries.FindSubscriptionsByChat(
			ctx, chatID,
		)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: msg - %v", repository.ErrUnknownDBError, err.Error())
		}
		domSubs := make([]domain.Subscription, len(subs))
		for i, sub := range subs {
			tags, err := queries.FindTagBySubscription(ctx, sub.ID)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf(
					"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
				)
			}
			tagsStr := make([]string, len(tags))
			for i, tag := range tags {
				tagsStr[i] = tag.Name
			}
			domSubs[i] = domain.NewSubscription(
				sub.ID, sub.LinkID, sub.ChatID, tagsStr,
			)
		}
		return domSubs, nil
	})
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
	return withTx(ctx, repo.pool, repo.queries, func(queries *gen.Queries) ([]domain.Subscription, error) {
		subs, err := queries.FindSubscriptionsByLink(ctx, link.ID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf(
				"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
			)
		}
		domSubs := make([]domain.Subscription, len(subs))
		for i, sub := range subs {
			tags, err := queries.FindTagBySubscription(ctx, sub.ID)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf(
					"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
				)
			}
			tagsStr := make([]string, len(tags))
			for i, tag := range tags {
				tagsStr[i] = tag.Name
			}
			domSubs[i] = domain.NewSubscription(
				sub.ID, sub.LinkID, sub.ChatID, tagsStr,
			)
		}
		return domSubs, nil
	})
}

func (repo *SubscriptionRepositorySQL) isChatRegistered(ctx context.Context, chatID int64) (bool, error) {
	_, err := repo.queries.GetChat(ctx, chatID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf(
			"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
		)
	}
	return true, nil
}

func (repo *SubscriptionRepositorySQL) getOrCreateLink(
	ctx context.Context, link domain.Link,
) (domain.Link, error) {
	dbLink, err := repo.queries.FindLinkByURL(ctx, link.URL.String())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			dbLink, err := repo.queries.AddLink(ctx, link.URL.String())
			if err != nil {
				return domain.Link{}, fmt.Errorf(
					"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
				)
			}
			return toDomainLink(dbLink)
		}
		return domain.Link{}, fmt.Errorf(
			"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
		)
	}
	return toDomainLink(dbLink)
}

func (repo *SubscriptionRepositorySQL) getOrCreateTag(
	ctx context.Context, tag string,
) (gen.Tag, error) {
	dbTag, err := repo.queries.FindTag(ctx, tag)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			dbTag, err = repo.queries.AddTag(ctx, tag)
			if err != nil {
				return gen.Tag{}, fmt.Errorf(
					"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
				)
			}
			return dbTag, nil
		}
		return gen.Tag{}, fmt.Errorf(
			"%w: msg - %v", repository.ErrUnknownDBError, err.Error(),
		)
	}
	return dbTag, nil
}
