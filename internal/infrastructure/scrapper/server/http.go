package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"link-tracker/internal/application/model"
	"link-tracker/internal/application/ports"
	"link-tracker/internal/infrastructure/openapi/scrapper"
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
	untrackLink := model.UntrackLinkCmd{
		ChatID: params.TgChatId,
		RawURL: removeData.Link,
	}
	deletedTracking, err := s.port.DeleteLink(r.Context(), untrackLink)
	if err != nil {
		s.log.Error(
			"can't delete link",
			"err", err, "for chat", params.TgChatId,
		)
		switch {
		case errors.Is(err, ports.ErrBadParams):
			http.Error(w, "invalid request parameters", http.StatusBadRequest)
		case errors.Is(err, ports.ErrNoChatWithThisID) || errors.Is(err, ports.ErrNoLink):
			http.Error(w, "no chat with this id or no such link", http.StatusNotFound)
		default:
			http.Error(w, "unexpected error", http.StatusInternalServerError)
		}
		return
	}

	var resp = scrapper.LinkResponse{
		Id:   deletedTracking.LinkID,
		Url:  deletedTracking.RawURL,
		Tags: deletedTracking.Tags,
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
	trackedLinks, err := s.port.GetTrackedLinks(
		r.Context(), model.ListTrackedLinksQuery{ChatID: chatID},
	)
	if err != nil {
		s.log.Error(
			"can't get links",
			"err", err, "for chat", params.TgChatId,
		)
		switch {
		case errors.Is(err, ports.ErrBadParams):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, ports.ErrNoChatWithThisID):
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	var resp scrapper.ListLinksResponse
	n := int32(len(trackedLinks))
	respLinks := make([]scrapper.LinkResponse, n)
	for i, link := range trackedLinks {
		respLinks[i] = scrapper.LinkResponse{
			Id:   link.LinkID,
			Url:  link.RawURL,
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

	addLinkDTO := model.TrackLinkCmd{
		ChatID: params.TgChatId,
		RawURL: linkData.Link,
		Tags:   linkData.Tags,
	}
	addedLink, err := s.port.AddLink(r.Context(), addLinkDTO)
	if err != nil {
		s.log.Error(
			"can't add new link",
			"err", err,
			"for chat", params.TgChatId,
			"url", linkData.Link,
		)
		switch {
		case errors.Is(err, ports.ErrBadParams):
			http.Error(w, "invalid request parameters", http.StatusBadRequest)
		case errors.Is(err, ports.ErrNoChatWithThisID):
			http.Error(w, "no such chat", http.StatusNotFound)
		case errors.Is(err, ports.ErrLinkAlreadyTracked):
			http.Error(w, "link already tracked", http.StatusConflict)
		default:
			http.Error(w, "unexpected error", http.StatusInternalServerError)
		}
		return
	}

	resp := scrapper.LinkResponse{
		Id:   addedLink.LinkID,
		Url:  addedLink.RawURL,
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
		switch {
		case errors.Is(err, ports.ErrBadParams):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, ports.ErrNoChatWithThisID):
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
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
		switch {
		case errors.Is(err, ports.ErrBadParams):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, ports.ErrChatAlreadyExist):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
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
