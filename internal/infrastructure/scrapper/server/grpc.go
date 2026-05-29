package server

import (
	"context"
	"errors"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"link-tracker/internal/application/model"
	"link-tracker/internal/application/ports"
	pb "link-tracker/internal/infrastructure/grpc/scrapper"
)

type ScrapperGRPCServer struct {
	logger *slog.Logger
	port   ports.ScrapperPort
	pb.UnimplementedScrapperServer
}

func NewScrapperGRPCServe(
	logger *slog.Logger, port ports.ScrapperPort,
) *ScrapperGRPCServer {
	return &ScrapperGRPCServer{
		logger: logger,
		port:   port,
	}
}

func (s *ScrapperGRPCServer) RegisterChat(
	ctx context.Context, in *pb.TgChatId,
) (*emptypb.Empty, error) {
	s.logger.Info("RegisterChat procedure was called")
	err := s.port.RegisterTGChat(ctx, in.GetId())
	if err != nil {
		s.logger.Error(
			"Failed to register chat",
			"err", err, "for chat", in.GetId(),
		)
		switch {
		case errors.Is(err, ports.ErrBadParams):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, ports.ErrChatAlreadyExist):
			return nil, status.Error(codes.AlreadyExists, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return &emptypb.Empty{}, nil
}

func (s *ScrapperGRPCServer) DeleteChat(
	ctx context.Context, in *pb.TgChatId,
) (*emptypb.Empty, error) {
	s.logger.Info("DeleteChat procedure was called")
	err := s.port.DeleteTGChat(ctx, in.GetId())
	if err != nil {
		s.logger.Error(
			"Failed to register chat",
			"err", err, "for chat", in.GetId(),
		)
		switch {
		case errors.Is(err, ports.ErrBadParams):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, ports.ErrNoChatWithThisID):
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return &emptypb.Empty{}, nil
}

func (s *ScrapperGRPCServer) GetTrackedLinks(
	ctx context.Context, in *pb.TgChatId,
) (*pb.ListLinksResponse, error) {
	s.logger.Info("GetTrackedLinks procedure was called")
	trackedLinks, err := s.port.GetTrackedLinks(ctx, model.ListTrackedLinksQuery{
		ChatID: in.GetId(),
	})
	if err != nil {
		s.logger.Error(
			"can't get links",
			"err", err, "for chat", in.GetId(),
		)
		if errors.Is(err, ports.ErrBadParams) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		} else if errors.Is(err, ports.ErrNoChatWithThisID) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	links := make([]*pb.LinkResponse, len(trackedLinks))
	for i, link := range trackedLinks {
		links[i] = &pb.LinkResponse{
			Id:   link.LinkID,
			Url:  link.RawURL,
			Tags: link.Tags,
		}
	}
	return &pb.ListLinksResponse{
		Size:  int32(len(trackedLinks)),
		Links: links,
	}, nil
}

func (s *ScrapperGRPCServer) AddLinkToTracking(
	ctx context.Context, in *pb.AddLinkRequest,
) (*pb.LinkResponse, error) {
	s.logger.Info("AddLinkToTracking procedure was called")
	trackedLink, err := s.port.AddLink(ctx, model.TrackLinkCmd{
		ChatID: in.GetChatId().GetId(),
		RawURL: in.GetLink(),
		Tags:   in.GetTags(),
	})
	if err != nil {
		s.logger.Error(
			"can't add new link",
			"err", err,
			"for chat", in.ChatId.GetId(),
			"url", in.Link,
		)
		switch {
		case errors.Is(err, ports.ErrBadParams):
			return nil, status.Error(codes.InvalidArgument, "invalid request parameters")
		case errors.Is(err, ports.ErrNoChatWithThisID):
			return nil, status.Error(codes.NotFound, "no such chat")
		case errors.Is(err, ports.ErrLinkAlreadyTracked):
			return nil, status.Error(codes.AlreadyExists, "link already tracked")
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return &pb.LinkResponse{
		Id:   trackedLink.LinkID,
		Url:  trackedLink.RawURL,
		Tags: trackedLink.Tags,
	}, nil
}

func (s *ScrapperGRPCServer) DeleteLinkFromTracking(
	ctx context.Context, in *pb.RemoveLinkRequest,
) (*pb.LinkResponse, error) {
	s.logger.Info("DeleteLinkFromTracking procedure was called")
	deletedLink, err := s.port.DeleteLink(ctx, model.UntrackLinkCmd{
		ChatID: in.Id.GetId(),
		RawURL: in.Link,
	})
	if err != nil {
		s.logger.Error(
			"can't delete link",
			"err", err, "for chat", in.Id.GetId(),
		)
		if errors.Is(err, ports.ErrBadParams) {
			return nil, status.Error(codes.InvalidArgument, "invalid request parameters")
		} else if errors.Is(err, ports.ErrNoChatWithThisID) {
			return nil, status.Error(codes.NotFound, "no such chat")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.LinkResponse{
		Id:   deletedLink.LinkID,
		Url:  deletedLink.RawURL,
		Tags: deletedLink.Tags,
	}, nil
}
