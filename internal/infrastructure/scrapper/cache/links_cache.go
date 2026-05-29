package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/valkey-io/valkey-go"
	"link-tracker/internal/application/model"
)

var UnknownCacheError = errors.New("unknown cache error")

type LinksCache interface {
	Get(ctx context.Context, chatID int64) ([]model.TrackedLinkView, bool, error)
	Set(ctx context.Context, chatID int64, links []model.TrackedLinkView) error
	Delete(ctx context.Context, chatID int64) error
}

type ValkeyLinksCache struct {
	logger *slog.Logger
	client valkey.Client
	ttl    time.Duration
}

func NewValkeyLinksCache(
	logger *slog.Logger, client valkey.Client, ttl time.Duration,
) LinksCache {
	return &ValkeyLinksCache{
		logger: logger,
		client: client,
		ttl:    ttl,
	}
}

func (c *ValkeyLinksCache) Get(ctx context.Context, chatID int64) ([]model.TrackedLinkView, bool, error) {
	var links []model.TrackedLinkView
	result := c.client.DoCache(ctx,
		c.client.B().Get().
			Key(strconv.Itoa(int(chatID))).
			Cache(), c.ttl)
	if result.IsCacheHit() {
		c.logger.Info("cleint side cache hit", "chat id", chatID)
	}
	err := result.DecodeJSON(&links)
	if err != nil {
		if errors.Is(err, valkey.Nil) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("%w: %w", UnknownCacheError, err)
	}
	return links, true, nil
}

func (c *ValkeyLinksCache) Set(ctx context.Context, chatID int64, links []model.TrackedLinkView) error {
	linksMsg, err := json.Marshal(links)
	if err != nil {
		return err
	}
	setQuery := c.client.B().Set().
		Key(strconv.Itoa(int(chatID))).
		Value(string(linksMsg))
	if c.ttl <= 0 {
		result := c.client.Do(ctx, setQuery.Build())
		return result.Error()
	}
	query := setQuery.Px(c.ttl).Build()
	result := c.client.Do(ctx, query)
	return result.Error()
}

func (c *ValkeyLinksCache) Delete(ctx context.Context, chatID int64) error {
	query := c.client.B().Del().
		Key(strconv.Itoa(int(chatID))).
		Build()
	result := c.client.Do(ctx, query)
	return result.Error()
}
