package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/openapi/scrapper"
)

type ScrapperServer struct {
	log  *slog.Logger
	port ports.ScrapperPort
}

func NewScrapperServer(l *slog.Logger, scrapperPort ports.ScrapperPort) *ScrapperServer {
	return &ScrapperServer{
		log:  l,
		port: scrapperPort,
	}
}

func (s *ScrapperServer) DeleteLinks(
	w http.ResponseWriter, r *http.Request, params scrapper.DeleteLinksParams,
) {
}

func (s *ScrapperServer) GetLinks(
	w http.ResponseWriter, r *http.Request, params scrapper.GetLinksParams,
) {
	chatID := params.TgChatId
	links, err := s.port.GetTrackedLinks(r.Context(), chatID)
	if err != nil {
		if errors.Is(err, ports.ErrBadParams) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else if errors.Is(err, ports.ErrNoChatWithThisID) {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	var resp scrapper.ListLinksResponse
	var n int32 = int32(len(links))
	respLinks := make([]scrapper.LinkResponse, n)
	for i, link := range links {
		respLinks[i] = scrapper.LinkResponse{
			Id:   &link.ID,
			Url:  &link.URL,
			Tags: &link.Tags,
		}
	}

	resp.Links = &respLinks
	resp.Size = &n

	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		slog.Error(
			"can't encode response",
			"err", err,
		)
	}
}

func (s *ScrapperServer) PostLinks(
	w http.ResponseWriter, r *http.Request, params scrapper.PostLinksParams,
) {
}

func (s *ScrapperServer) DeleteTgChatId(
	w http.ResponseWriter, r *http.Request, id int64,
) {
	err := s.port.DeleteTGChat(r.Context(), id)
	if err != nil {
		slog.Error(
			"can't delete chat",
			"err", err,
			"for chat", id,
		)
		if errors.Is(err, ports.ErrBadParams) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else if errors.Is(err, ports.ErrNoChatWithThisID) {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			s.log.Warn(
				"unexpeted error type or value",
				"err", err,
			)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *ScrapperServer) PostTgChatId(
	w http.ResponseWriter, r *http.Request, id int64,
) {
	err := s.port.RegisterTGChat(r.Context(), id)
	if err != nil {
		s.log.Error(
			"can't proceed char registration",
			"err", err,
			"for chat", id,
		)
		if errors.Is(err, ports.ErrBadParams) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else if errors.Is(err, ports.ErrChatAlreadyExist) {
			http.Error(w, err.Error(), http.StatusConflict)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			s.log.Warn(
				"unexpeted error type or value",
				"err", err,
			)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
}
