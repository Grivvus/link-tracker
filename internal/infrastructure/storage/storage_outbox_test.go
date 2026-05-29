package storage

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"link-tracker/internal/application/model"
)

func TestStorage_NotificationOutboxLifecycle(t *testing.T) {
	ctx := context.Background()
	st := New()
	const chatID int64 = 101

	mustRegisterChat(t, st, chatID)

	link := mustNewLink(t, "https://github.com/acme/project-outbox")
	sub, err := st.Subscribe(ctx, link, chatID, []string{"alerts"})
	require.NoError(t, err)

	storedLink, err := st.GetLink(ctx, sub.LinkID)
	require.NoError(t, err)

	ts1 := time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC)
	ts2 := ts1.Add(10 * time.Minute)
	ts3 := ts2.Add(10 * time.Minute)
	storedLink.LastUpdate = ts3

	_, err = st.UpdateLink(ctx, storedLink, []model.LinkUpdatedEvent{
		{
			LinkID:       storedLink.ID,
			RawURL:       storedLink.URL.String(),
			UpdatedAt:    ts1,
			UpdateName:   "issue-1",
			UpdateAuthor: "alice",
			Description:  "first event",
			ChatIDs:      []int64{chatID},
		},
		{
			LinkID:       storedLink.ID,
			RawURL:       storedLink.URL.String(),
			UpdatedAt:    ts2,
			UpdateName:   "issue-2",
			UpdateAuthor: "bob",
			Description:  "second event",
			ChatIDs:      []int64{chatID},
		},
		{
			LinkID:       storedLink.ID,
			RawURL:       storedLink.URL.String(),
			UpdatedAt:    ts3,
			UpdateName:   "issue-3",
			UpdateAuthor: "carol",
			Description:  "third event",
			ChatIDs:      []int64{chatID},
		},
	})
	require.NoError(t, err)

	pending, err := st.GetPending(ctx, 0, 2)
	require.NoError(t, err)
	require.Len(t, pending, 2)
	assert.Equal(t, []int64{1, 2}, outboxEventIDs(pending))
	assert.Equal(t, []int64{chatID}, pending[0].Event.ChatIDs)
	assert.Equal(t, "issue-1", pending[0].Event.UpdateName)

	pending, err = st.GetPending(ctx, 1, 10)
	require.NoError(t, err)
	require.Len(t, pending, 2)
	assert.Equal(t, []int64{2, 3}, outboxEventIDs(pending))

	require.NoError(t, st.MarkSent(ctx, []int64{1, 3}))
	require.NoError(t, st.MarkFailed(ctx, 2, "delivery failed"))

	pending, err = st.GetPending(ctx, 0, 10)
	require.NoError(t, err)
	assert.Empty(t, pending)

	updatedLink, err := st.GetLink(ctx, storedLink.ID)
	require.NoError(t, err)
	assert.Equal(t, ts3, updatedLink.LastUpdate)
}

func outboxEventIDs(events []model.OutboxLinkUpdateEvent) []int64 {
	ids := make([]int64, len(events))
	for i, event := range events {
		ids[i] = event.ID
	}
	return ids
}
