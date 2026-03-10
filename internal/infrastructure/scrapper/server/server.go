package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/dto"
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
	s.log.Info("DeleteLinks handler called", "for chat", params.TgChatId)
	var removeData scrapper.RemoveLinkRequest
	err := json.NewDecoder(r.Body).Decode(&removeData)
	if err != nil {
		s.log.Error("can't decode json body", "err", err)
		http.Error(w, "invalid request parameters", http.StatusBadRequest)
		return
	}
	deleteDTO := dto.DeleteLink{
		ChatID: params.TgChatId,
		Link:   removeData.Link,
	}
	respDTO, err := s.port.DeleteLink(r.Context(), deleteDTO)
	if err != nil {
		s.log.Error(
			"can't delete link",
			"err", err, "for chat", params.TgChatId,
		)
		if errors.Is(err, ports.ErrBadParams) {
			http.Error(w, "invalid request parameters", http.StatusBadRequest)
		} else if errors.Is(err, ports.ErrNoChatWithThisID) || errors.Is(err, ports.ErrNoLink) {
			http.Error(w, "no chat with this id or no such link", http.StatusNotFound)
		} else {
			http.Error(w, "unexpected error", http.StatusInternalServerError)
		}
		return
	}

	var resp = scrapper.LinkResponse{
		Id:   respDTO.ID,
		Url:  respDTO.URL,
		Tags: respDTO.Tags,
	}
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		s.log.Error(
			"can't encode response",
			"err", err,
			"for chat", params.TgChatId,
		)
	}
	w.WriteHeader(http.StatusOK)
}

func (s *ScrapperServer) GetLinks(
	w http.ResponseWriter, r *http.Request, params scrapper.GetLinksParams,
) {
	s.log.Info("GetLinks handler called", "for chat", params.TgChatId)
	chatID := params.TgChatId
	links, err := s.port.GetTrackedLinks(r.Context(), chatID)
	if err != nil {
		s.log.Error(
			"can't get links",
			"err", err, "for chat", params.TgChatId,
		)
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
			Id:   link.ID,
			Url:  link.URL,
			Tags: link.Tags,
		}
	}

	resp.Links = respLinks
	resp.Size = n

	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		s.log.Error(
			"can't encode response",
			"err", err,
		)
	}
}

func (s *ScrapperServer) PostLinks(
	w http.ResponseWriter, r *http.Request, params scrapper.PostLinksParams,
) {
	s.log.Info("PostLinks handler called", "for chat", params.TgChatId)
	var linkData scrapper.AddLinkRequest
	err := json.NewDecoder(r.Body).Decode(&linkData)
	if err != nil {
		s.log.Error("can't decode json body", "err", err)
		http.Error(w, "invalid request parameters", http.StatusBadRequest)
		return
	}

	addLinkDTO := dto.AddLink{
		ChatID: params.TgChatId,
		Link:   linkData.Link,
		Tags:   linkData.Tags,
	}
	addedLink, err := s.port.AddLink(r.Context(), addLinkDTO)
	if err != nil {
		s.log.Error(
			"can't add new link",
			"err", err,
			"for chat", params.TgChatId,
		)
		if errors.Is(err, ports.ErrBadParams) {
			http.Error(w, "invalid request parameters", http.StatusBadRequest)
		} else if errors.Is(err, ports.ErrNoChatWithThisID) {
			http.Error(w, "no such chat", http.StatusNotFound)
		} else if errors.Is(err, ports.ErrLinkAlreadyTracked) {
			http.Error(w, "link already tracked", http.StatusConflict)
		} else {
			http.Error(w, "unexpected error", http.StatusInternalServerError)
		}
		return
	}

	resp := scrapper.LinkResponse{
		Id:   addedLink.ID,
		Url:  addedLink.URL,
		Tags: addedLink.Tags,
	}
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		s.log.Error(
			"can't encode response",
			"err", err,
			"for chat", params.TgChatId,
		)
	}
	w.WriteHeader(http.StatusOK)
}

func (s *ScrapperServer) DeleteTgChatId(
	w http.ResponseWriter, r *http.Request, id int64,
) {
	s.log.Info("DeleteTgChatId handler called", "for chat", id)
	err := s.port.DeleteTGChat(r.Context(), id)
	if err != nil {
		s.log.Error(
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
				"unexpected error type or value",
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
	s.log.Info("PostTgChatId handler called", "for chat", id)
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
				"unexpected error type or value",
				"err", err,
			)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
}
