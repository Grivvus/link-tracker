package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
)

type LinkPollingUseCase struct {
	sched              gocron.Scheduler
	logger             *slog.Logger
	linkState          ports.LinkStatePort
	botPort            ports.BotPort
	trackingRepository repository.TrackingRepository
}

func NewLinkPollingUseCase(
	logger *slog.Logger, sched gocron.Scheduler,
	linkState ports.LinkStatePort, botPort ports.BotPort,
	trackingRepository repository.TrackingRepository,
) LinkPollingUseCase {
	return LinkPollingUseCase{
		sched:              sched,
		logger:             logger,
		linkState:          linkState,
		botPort:            botPort,
		trackingRepository: trackingRepository,
	}
}

func (poller *LinkPollingUseCase) Start(ctx context.Context) {
	_, err := poller.sched.NewJob(
		gocron.DurationJob(1*time.Hour),
		gocron.NewTask(func() {
			trackings, err := poller.trackingRepository.GetAllTrackings(ctx)
			if err != nil {
				poller.logger.Error("can't fetch trackings", "err", err)
				return
			}
			_ = trackings
		}),
	)
	if err != nil {
		poller.logger.Error("can't create job", "err", err)
		return
	}
	poller.sched.Start()
}
