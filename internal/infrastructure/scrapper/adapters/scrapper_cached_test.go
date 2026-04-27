package adapters_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/scrapper/adapters"
)

type testCache struct {
	cache       map[int64][]model.TrackedLinkView
	getCalls    int
	setCalls    int
	deleteCalls int
}

func (c *testCache) Get(ctx context.Context, chatID int64) ([]model.TrackedLinkView, bool, error) {
	c.getCalls++
	links, ok := c.cache[chatID]
	return cloneTrackedLinks(links), ok, nil
}

func (c *testCache) Set(ctx context.Context, chatID int64, links []model.TrackedLinkView) error {
	c.setCalls++
	c.cache[chatID] = cloneTrackedLinks(links)
	return nil
}

func (c *testCache) Delete(ctx context.Context, chatID int64) error {
	c.deleteCalls++
	delete(c.cache, chatID)
	return nil
}

type testScrapperPort struct {
	linksByChat map[int64][]model.TrackedLinkView
	getCalls    int
	addCalls    int
	deleteCalls int
}

func (p *testScrapperPort) DeleteLink(
	ctx context.Context, cmd model.UntrackLinkCmd,
) (model.TrackedLinkView, error) {
	p.deleteCalls++
	links := p.linksByChat[cmd.ChatID]
	for i, link := range links {
		if link.RawURL == cmd.RawURL {
			p.linksByChat[cmd.ChatID] = append(
				cloneTrackedLinks(links[:i]),
				cloneTrackedLinks(links[i+1:])...,
			)
			return link, nil
		}
	}
	return model.TrackedLinkView{}, nil
}

func (p *testScrapperPort) GetTrackedLinks(
	ctx context.Context, query model.ListTrackedLinksQuery,
) ([]model.TrackedLinkView, error) {
	p.getCalls++
	return cloneTrackedLinks(p.linksByChat[query.ChatID]), nil
}

func (p *testScrapperPort) AddLink(
	ctx context.Context, cmd model.TrackLinkCmd,
) (model.TrackedLinkView, error) {
	p.addCalls++
	link := model.TrackedLinkView{
		LinkID: int64(len(p.linksByChat[cmd.ChatID]) + 1),
		RawURL: cmd.RawURL,
		Tags:   append([]string(nil), cmd.Tags...),
	}
	p.linksByChat[cmd.ChatID] = append(p.linksByChat[cmd.ChatID], link)
	return link, nil
}

func (p *testScrapperPort) DeleteTGChat(ctx context.Context, chatID int64) error {
	delete(p.linksByChat, chatID)
	return nil
}

func (p *testScrapperPort) RegisterTGChat(ctx context.Context, chatID int64) error {
	if _, ok := p.linksByChat[chatID]; !ok {
		p.linksByChat[chatID] = []model.TrackedLinkView{}
	}
	return nil
}

func TestCachedAdapter_AddLinkInvalidatesCacheAndNextGetUsesDB(t *testing.T) {
	ctx := context.Background()
	chatID := int64(1)
	staleLinks := []model.TrackedLinkView{{
		LinkID: 1,
		RawURL: "https://github.com/user/old",
		Tags:   []string{"stale"},
	}}
	addedRawURL := "https://github.com/user/new"

	cache := &testCache{
		cache: map[int64][]model.TrackedLinkView{
			chatID: staleLinks,
		},
	}
	db := &testScrapperPort{
		linksByChat: map[int64][]model.TrackedLinkView{
			chatID: cloneTrackedLinks(staleLinks),
		},
	}
	adapter := adapters.NewScrapperCachedAdapter(testLogger(), db, cache)

	added, err := adapter.AddLink(ctx, model.TrackLinkCmd{
		ChatID: chatID,
		RawURL: addedRawURL,
		Tags:   []string{"go"},
	})
	require.NoError(t, err)
	assert.Equal(t, addedRawURL, added.RawURL)
	assert.Equal(t, 1, db.addCalls)
	assert.Equal(t, 1, cache.deleteCalls)
	assert.NotContains(t, cache.cache, chatID)

	links, err := adapter.GetTrackedLinks(ctx, model.ListTrackedLinksQuery{ChatID: chatID})
	require.NoError(t, err)

	require.Len(t, links, 2)
	assert.Equal(t, staleLinks[0], links[0])
	assert.Equal(t, addedRawURL, links[1].RawURL)
	assert.Equal(t, 1, db.getCalls, "cache miss after invalidation should read from DB")
	assert.Equal(t, 1, cache.setCalls, "DB response should be written to cache")
	assert.Equal(t, links, cache.cache[chatID])
}

func TestCachedAdapter_DeleteLinkInvalidatesCacheAndNextGetUsesDB(t *testing.T) {
	ctx := context.Background()
	chatID := int64(2)
	deletedRawURL := "https://github.com/user/deleted"
	remainingLink := model.TrackedLinkView{
		LinkID: 2,
		RawURL: "https://github.com/user/remaining",
		Tags:   []string{"keep"},
	}
	dbLinks := []model.TrackedLinkView{
		{
			LinkID: 1,
			RawURL: deletedRawURL,
			Tags:   []string{"drop"},
		},
		remainingLink,
	}

	cache := &testCache{
		cache: map[int64][]model.TrackedLinkView{
			chatID: cloneTrackedLinks(dbLinks),
		},
	}
	db := &testScrapperPort{
		linksByChat: map[int64][]model.TrackedLinkView{
			chatID: cloneTrackedLinks(dbLinks),
		},
	}
	adapter := adapters.NewScrapperCachedAdapter(testLogger(), db, cache)

	deleted, err := adapter.DeleteLink(ctx, model.UntrackLinkCmd{
		ChatID: chatID,
		RawURL: deletedRawURL,
	})
	require.NoError(t, err)
	assert.Equal(t, deletedRawURL, deleted.RawURL)
	assert.Equal(t, 1, db.deleteCalls)
	assert.Equal(t, 1, cache.deleteCalls)
	assert.NotContains(t, cache.cache, chatID)

	links, err := adapter.GetTrackedLinks(ctx, model.ListTrackedLinksQuery{ChatID: chatID})
	require.NoError(t, err)

	assert.Equal(t, []model.TrackedLinkView{remainingLink}, links)
	assert.Equal(t, 1, db.getCalls, "cache miss after invalidation should read from DB")
	assert.Equal(t, 1, cache.setCalls, "DB response should be written to cache")
	assert.Equal(t, links, cache.cache[chatID])
}

func TestCachedAdapter_GetTrackedLinksFillsCacheAndNextGetUsesCache(t *testing.T) {
	ctx := context.Background()
	chatID := int64(3)
	dbLinks := []model.TrackedLinkView{
		{
			LinkID: 1,
			RawURL: "https://stackoverflow.com/questions/1",
			Tags:   []string{"so"},
		},
	}

	cache := &testCache{cache: map[int64][]model.TrackedLinkView{}}
	db := &testScrapperPort{
		linksByChat: map[int64][]model.TrackedLinkView{
			chatID: cloneTrackedLinks(dbLinks),
		},
	}
	adapter := adapters.NewScrapperCachedAdapter(testLogger(), db, cache)

	firstLinks, err := adapter.GetTrackedLinks(ctx, model.ListTrackedLinksQuery{ChatID: chatID})
	require.NoError(t, err)
	assert.Equal(t, dbLinks, firstLinks)
	assert.Equal(t, 1, db.getCalls)
	assert.Equal(t, 1, cache.setCalls)
	assert.Equal(t, dbLinks, cache.cache[chatID])

	db.linksByChat[chatID] = []model.TrackedLinkView{
		{
			LinkID: 2,
			RawURL: "https://github.com/user/not-used",
			Tags:   []string{"db"},
		},
	}

	secondLinks, err := adapter.GetTrackedLinks(ctx, model.ListTrackedLinksQuery{ChatID: chatID})
	require.NoError(t, err)

	assert.Equal(t, firstLinks, secondLinks)
	assert.Equal(t, 1, db.getCalls, "second request should use cache instead of DB")
	assert.Equal(t, 1, cache.setCalls, "cache hit should not rewrite cache")
}

func TestCachedAdapter_GetTrackedLinksCacheHitDoesNotCallNext(t *testing.T) {
	ctx := context.Background()
	chatID := int64(4)
	cachedLinks := []model.TrackedLinkView{
		{
			LinkID: 1,
			RawURL: "https://github.com/user/from-cache",
			Tags:   []string{"cache"},
		},
	}

	cache := &testCache{
		cache: map[int64][]model.TrackedLinkView{
			chatID: cloneTrackedLinks(cachedLinks),
		},
	}
	db := &testScrapperPort{
		linksByChat: map[int64][]model.TrackedLinkView{
			chatID: {
				{
					LinkID: 2,
					RawURL: "https://github.com/user/from-db",
					Tags:   []string{"db"},
				},
			},
		},
	}
	adapter := adapters.NewScrapperCachedAdapter(testLogger(), db, cache)

	links, err := adapter.GetTrackedLinks(ctx, model.ListTrackedLinksQuery{ChatID: chatID})
	require.NoError(t, err)

	assert.Equal(t, cachedLinks, links)
	assert.Equal(t, 1, cache.getCalls)
	assert.Equal(t, 0, db.getCalls, "cache hit should not call next")
	assert.Equal(t, 0, cache.setCalls, "cache hit should not rewrite cache")
}

func cloneTrackedLinks(links []model.TrackedLinkView) []model.TrackedLinkView {
	if links == nil {
		return nil
	}
	cloned := make([]model.TrackedLinkView, len(links))
	for i, link := range links {
		cloned[i] = link
		cloned[i].Tags = append([]string(nil), link.Tags...)
	}
	return cloned
}
