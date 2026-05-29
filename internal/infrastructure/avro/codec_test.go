//go:build integrational
// +build integrational

package avro

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"

	"github.com/linkedin/goavro/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"link-tracker/internal/application/model"
	"link-tracker/internal/infrastructure/openapi/bot"
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
		Priority:     model.PriorityHigh,
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
	assert.Equal(t, testModel.Priority, testModelDecoded.Priority)
	assert.Equal(t, testModel.UpdatedAt.Format(time.RFC3339), testModelDecoded.UpdatedAt.Format(time.RFC3339))
}

func TestCodec_DecodeReturnsErrorForInvalidUpdatedAt(t *testing.T) {
	t.Parallel()

	schema, err := getSchemaFromFile()
	require.NoError(t, err)
	avroCodec, err := goavro.NewCodec(schema)
	require.NoError(t, err)
	codec := &Codec{
		codec:    avroCodec,
		schemaID: 66,
	}
	badUpdate := bot.LinkUpdate{
		Id:           123,
		Url:          "github.com/linkedin/goavro",
		UpdatedAt:    "not-a-rfc3339-time",
		Title:        "new version",
		UpdateAuthor: "vasia pupkin",
		Description:  "new major version has been released",
		TgChatIds:    []int64{10, 11, 12},
		Priority:     bot.Medium,
	}
	jsonEncoded, err := json.Marshal(badUpdate)
	require.NoError(t, err)
	native, _, err := avroCodec.NativeFromTextual(jsonEncoded)
	require.NoError(t, err)
	binaryEncoded, err := avroCodec.BinaryFromNative(nil, native)
	require.NoError(t, err)

	msg := make([]byte, 0, 5+len(binaryEncoded))
	msg = append(msg, byte(0))
	schemaIDBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(schemaIDBytes, uint32(codec.schemaID))
	msg = append(msg, schemaIDBytes...)
	msg = append(msg, binaryEncoded...)

	_, err = codec.Decode(msg)
	require.Error(t, err)
	require.ErrorIs(t, err, DeserializationError)
	assert.ErrorContains(t, err, "cannot parse")
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
