package avro

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/linkedin/goavro/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
)

func TestCodec_DecodeEncodeGivesTheSameResult(t *testing.T) {
	t.Parallel()

	schema, err := getSchemaFromFile()
	require.NoError(t, err)
	avroCodec, err := goavro.NewCodec(schema)
	require.NoError(t, err)
	codec := &Codec{
		codec:    avroCodec,
		schemaID: 66,
	}
	testModel := model.LinkUpdatedEvent{
		LinkID:       int64(123),
		RawURL:       "github.com/linkedin/goavro",
		UpdatedAt:    time.Now(),
		UpdateName:   "new version",
		UpdateAuthor: "vasia pupkin",
		Description:  "new major version has been released",
		ChatIDs:      []int64{int64(10), int64(11), int64(12)},
	}
	encoded, err := codec.Encode(testModel)
	require.NoError(t, err)
	testModelDecoded, err := codec.Decode(encoded)
	require.NoError(t, err)
	assert.Equal(t, testModel.ChatIDs, testModelDecoded.ChatIDs)
	assert.Equal(t, testModel.UpdateName, testModelDecoded.UpdateName)
	assert.Equal(t, testModel.UpdateAuthor, testModelDecoded.UpdateAuthor)
	assert.Equal(t, testModel.Description, testModelDecoded.Description)
	assert.Equal(t, testModel.LinkID, testModelDecoded.LinkID)
	assert.Equal(t, testModel.RawURL, testModelDecoded.RawURL)
	assert.Equal(t, testModel.UpdatedAt.Format(time.RFC3339), testModelDecoded.UpdatedAt.Format(time.RFC3339))
}

func getSchemaFromFile() (string, error) {
	f, err := os.Open("../../../avro/link_update.avsc")
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	read, err := io.ReadAll(f)
	return string(read), err
}
