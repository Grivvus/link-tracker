package avro

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/linkedin/goavro/v2"
	"github.com/riferrei/srclient"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/bot"
)

var SerializationError = errors.New("serialization error")
var DeserializationError = errors.New("deserialization error")

type Codec struct {
	codec    *goavro.Codec
	schema   *srclient.Schema
	schemaID int
}

func NewCodecFromSchema(schema string, schemaID int) (*Codec, error) {
	avroCodec, err := goavro.NewCodec(schema)
	if err != nil {
		return nil, err
	}
	return &Codec{
		codec:    avroCodec,
		schemaID: schemaID,
	}, nil
}

func NewCodec(
	schemaRegistryURL string, topic string, registryFilename string,
) (*Codec, error) {
	client := srclient.NewSchemaRegistryClient(schemaRegistryURL)
	schema, _ := client.GetLatestSchema(topic)
	if schema == nil {
		schemaFile, err := os.Open(registryFilename)
		if err != nil {
			return nil, err
		}
		defer func() { _ = schemaFile.Close() }()
		schemaBytes, err := io.ReadAll(schemaFile)
		if err != nil {
			return nil, err
		}
		schema, err = client.CreateSchema(topic, string(schemaBytes), srclient.Avro)
		if err != nil {
			return nil, err
		}
	}
	return &Codec{
		codec:    schema.Codec(),
		schema:   schema,
		schemaID: schema.ID(),
	}, nil
}

func (c *Codec) Encode(msg model.LinkUpdatedEvent) ([]byte, error) {
	dto := modelToDTO(msg)
	jencoded, err := json.Marshal(dto)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", SerializationError, err)
	}
	native, _, err := c.codec.NativeFromTextual(jencoded)
	bytes, err := c.codec.BinaryFromNative(nil, native)
	schemaIDBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(schemaIDBytes, uint32(c.schemaID))

	var recordValue []byte
	recordValue = append(recordValue, byte(0))
	recordValue = append(recordValue, schemaIDBytes...)
	recordValue = append(recordValue, bytes...)
	return recordValue, nil

}

func (c *Codec) Decode(msg []byte) (model.LinkUpdatedEvent, error) {
	if len(msg) < 5 {
		return model.LinkUpdatedEvent{}, fmt.Errorf("%w: message is too short", DeserializationError)
	}
	native, _, err := c.codec.NativeFromBinary(msg[5:])
	if err != nil {
		return model.LinkUpdatedEvent{}, fmt.Errorf("%w: %w", DeserializationError, err)
	}
	jsonVal, err := c.codec.TextualFromNative(nil, native)
	if err != nil {
		return model.LinkUpdatedEvent{}, fmt.Errorf("%w: %w", DeserializationError, err)
	}
	var update bot.LinkUpdate
	err = json.Unmarshal(jsonVal, &update)
	if err != nil {
		return model.LinkUpdatedEvent{}, fmt.Errorf("%w: %w", DeserializationError, err)
	}
	modelUpdate, err := dtoToModel(update)
	if err != nil {
		return model.LinkUpdatedEvent{}, fmt.Errorf("%w: %w", DeserializationError, err)
	}
	return modelUpdate, nil
}

func dtoToModel(dto bot.LinkUpdate) (model.LinkUpdatedEvent, error) {
	updatedAt, err := time.Parse(time.RFC3339, dto.UpdatedAt)
	if err != nil {
		return model.LinkUpdatedEvent{}, err
	}
	return model.LinkUpdatedEvent{
		Description:  dto.Description,
		UpdateAuthor: dto.UpdateAuthor,
		UpdatedAt:    updatedAt,
		UpdateName:   dto.Title,
		RawURL:       dto.Url,
		ChatIDs:      dto.TgChatIds,
		LinkID:       dto.Id,
	}, nil
}

func modelToDTO(upd model.LinkUpdatedEvent) bot.LinkUpdate {
	return bot.LinkUpdate{
		Id:           upd.LinkID,
		Description:  upd.Description,
		TgChatIds:    upd.ChatIDs,
		Url:          upd.RawURL,
		Title:        upd.UpdateName,
		UpdateAuthor: upd.UpdateAuthor,
		UpdatedAt:    upd.UpdatedAt.Format(time.RFC3339),
	}
}
