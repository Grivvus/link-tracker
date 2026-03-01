package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/bot"
)

type BotServer struct {
	logger *slog.Logger
}

func New(l *slog.Logger) *BotServer {
	return &BotServer{
		logger: l,
	}
}

func (serv *BotServer) PostUpdates(w http.ResponseWriter, r *http.Request) {
	serv.logger.Info("PostUpdates called")
	var updates bot.PostUpdatesJSONRequestBody
	err := json.NewDecoder(r.Body).Decode(&updates)
	if err != nil {
		http.Error(w, "can't decode body", http.StatusBadRequest)
		return
	}
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
