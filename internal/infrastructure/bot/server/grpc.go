package server

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	pb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/grpc/bot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type BotGRPCServer struct {
	logger *slog.Logger
	port   ports.BotPort
	pb.UnimplementedBotServer
}

func NewBotGRPCServer(logger *slog.Logger, port ports.BotPort) *BotGRPCServer {
	return &BotGRPCServer{
		logger: logger,
		port:   port,
	}
}

func (s *BotGRPCServer) NotifyAboutUpdate(
	_ context.Context, in *pb.LinkUpdate,
) (*emptypb.Empty, error) {
	updatedAt, err := time.Parse(time.RFC3339, in.UpdatedAt)
	if err != nil {
		return &emptypb.Empty{}, status.Error(codes.InvalidArgument, "can't parse updatedAt field")
	}
	// passed ctx was already canceled
	err = s.port.NotifyAboutUpdatedLinks(context.Background(), model.LinkUpdatedEvent{
		LinkID:       in.Id,
		RawURL:       in.Url,
		UpdateAuthor: in.UpdateAuthor,
		UpdatedAt:    updatedAt,
		UpdateName:   in.Title,
		Description:  in.Description,
		ChatIDs:      in.TgChatIds,
	})
	if err != nil {
		s.logger.Error("can't send notifications", "err", err)
		if errors.Is(err, ports.ErrBadParams) {
			return &emptypb.Empty{}, status.Error(codes.InvalidArgument, "bad request params")
		}
		return &emptypb.Empty{}, status.Error(codes.Internal, "server internal error")
	}
	return &emptypb.Empty{}, nil
}
