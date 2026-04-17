package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
)

type OutboxRelayUseCase struct {
	logger     *slog.Logger
	sched      gocron.Scheduler
	sender     ports.BotPort
	outboxRepo repository.NotificationOutboxRepository
	limit      int
}

func NewOutboxRelayUseCase(
	logger *slog.Logger, sender ports.BotPort,
	repo repository.NotificationOutboxRepository,
	scheduler gocron.Scheduler,
	pendingLimit int,
) *OutboxRelayUseCase {
	return &OutboxRelayUseCase{
		logger:     logger,
		sender:     sender,
		outboxRepo: repo,
		limit:      pendingLimit,
		sched:      scheduler,
	}
}

func (uc *OutboxRelayUseCase) Start(ctx context.Context, period time.Duration) (gocron.Job, error) {
	job, err := uc.sched.NewJob(
		gocron.DurationJob(period),
		gocron.NewTask(func() {
			// can i do something with this err?
			_ = uc.workerFn(ctx)
		}),
	)
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (uc *OutboxRelayUseCase) workerFn(ctx context.Context) error {
	hasMore := true
	startsWith := int64(0)
	for hasMore {
		events, err := uc.outboxRepo.GetPending(ctx, startsWith, int32(uc.limit))
		if err != nil {
			return err
		}
		if len(events) < uc.limit {
			hasMore = false
		}
		succeeded := make([]int64, 0, len(events))
		for _, outboxEvent := range events {
			startsWith = outboxEvent.ID
			err := uc.sender.NotifyAboutUpdatedLinks(ctx, outboxEvent.Event)
			if err != nil {
				err := uc.outboxRepo.MarkFailed(ctx, outboxEvent.ID, err.Error())
				if err != nil {
					uc.logger.Error("can't mark outbox failed", "err", err)
				}
				continue
			}
			succeeded = append(succeeded, outboxEvent.ID)
		}
		err = uc.outboxRepo.MarkSent(ctx, succeeded)
		if err != nil {
			uc.logger.Error("can't mark outbox sent", "err", err)
			return err
		}
	}
	return nil
}
