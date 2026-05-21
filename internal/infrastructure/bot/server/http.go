package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/bot"
)

type BotServer struct {
	logger *slog.Logger
	port   ports.BotPort
}

func New(l *slog.Logger, port ports.BotPort) *BotServer {
	return &BotServer{
		logger: l,
		port:   port,
	}
}

func (serv *BotServer) PostUpdates(w http.ResponseWriter, r *http.Request) {
	serv.logger.Info("PostUpdates handler called")
	var updates bot.PostUpdatesJSONRequestBody
	err := json.NewDecoder(r.Body).Decode(&updates)
	if err != nil {
		http.Error(w, "can't decode body", http.StatusBadRequest)
		return
	}
	updatedAt, err := time.Parse(time.RFC3339, updates.UpdatedAt)
	if err != nil {
		http.Error(w, "can't parse updatedAt field", http.StatusBadRequest)
	}
	// if we're pass request context, it will be canceled before bot send the message
	err = serv.port.NotifyAboutUpdatedLinks(context.Background(), model.LinkUpdatedEvent{
		LinkID:       updates.Id,
		ChatIDs:      updates.TgChatIds,
		RawURL:       updates.Url,
		Description:  updates.Description,
		UpdateAuthor: updates.UpdateAuthor,
		UpdatedAt:    updatedAt,
		UpdateName:   updates.Title,
		Priority:     model.PriorityFromString(string(updates.Priority)),
	})
	if err != nil {
		if errors.Is(err, ports.ErrBadParams) {
			http.Error(w, "bad request parameters", http.StatusBadRequest)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
}
