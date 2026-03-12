package bot

import (
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func draftTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestDialogDraftsStorage_AddURL_InvalidURL(t *testing.T) {
	t.Parallel()

	st := newDialogDraftsStorage(draftTestLogger())
	const chatID int64 = 11

	require.NoError(t, st.StartFilling(chatID))
	assert.Error(t, st.AddURL(chatID, "abcd://github.com/user/repo"))
}

func TestDialogDraftsStorage_FullTrackDraft(t *testing.T) {
	st := newDialogDraftsStorage(draftTestLogger())
	const chatID int64 = 12

	require.NoError(t, st.StartFilling(chatID))
	require.NoError(t, st.AddURL(chatID, "https://github.com/user/repo"))
	require.NoError(t, st.AddTags(chatID, []string{"go", "backend"}))

	cmd, err := st.GetFilledTracking(chatID)
	require.NoError(t, err)
	assert.Equal(t, chatID, cmd.ChatID)
	assert.Equal(t, "https://github.com/user/repo", cmd.RawURL)
	assert.Len(t, cmd.Tags, 2)
}
