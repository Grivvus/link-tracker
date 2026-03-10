package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/dto"
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
	// if we're pass request context, it will be canceled before bot send the message
	err = serv.port.NotifyAboutUpdatedLinks(context.Background(), dto.LinkUpdate{
		ID:          updates.Id,
		TGChatIDs:   updates.TgChatIds,
		URL:         updates.Url,
		Description: updates.Description,
	})
	if err != nil {
		if errors.Is(err, ports.ErrBadParams) {
			http.Error(w, "bad request parameters", http.StatusBadRequest)
		} else {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}
}
