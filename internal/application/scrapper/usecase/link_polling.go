package usecase

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type LinkPollingUseCase struct {
	sched                  gocron.Scheduler
	logger                 *slog.Logger
	linkState              ports.LinkStatePort
	subscriptionRepository repository.SubscriptionRepository
	pollerIntervalMinutes  int64
}

func NewLinkPollingUseCase(
	logger *slog.Logger, sched gocron.Scheduler,
	linkState ports.LinkStatePort,
	subscriptionRepo repository.SubscriptionRepository,
	pollerIntervalMinutes int64,
) LinkPollingUseCase {
	return LinkPollingUseCase{
		sched:                  sched,
		logger:                 logger,
		linkState:              linkState,
		subscriptionRepository: subscriptionRepo,
		pollerIntervalMinutes:  pollerIntervalMinutes,
	}
}

func (poller *LinkPollingUseCase) Start(
	ctx context.Context, concurrencyBoundary uint,
) (gocron.Job, error) {
	job, err := poller.sched.NewJob(
		gocron.DurationJob(time.Duration(poller.pollerIntervalMinutes)*time.Minute),
		gocron.NewTask(func() {
			var wg sync.WaitGroup
			poller.logger.Info("Start polling job")
			links, errs := poller.subscriptionRepository.GetAllLinks(ctx)
			go func() {
				for err := range errs {
					poller.logger.Error("can't fetch links", "err", err)
				}
			}()
			for range concurrencyBoundary {
				wg.Go(func() {
					poller.linkWorkerFn(ctx, links)
				})
			}
			wg.Wait()
		}),
	)
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (poller *LinkPollingUseCase) linkWorkerFn(
	ctx context.Context, links <-chan domain.Link,
) {
	for link := range links {
		poller.logger.Info("Fetch link", "url", link.URL)
		subs, err := poller.subscriptionRepository.AllLinkSubscriptions(ctx, link)
		if err != nil {
			poller.logger.Warn("can't fetch trackings", "err", err)
			continue
		}
		subsChatIDs := make([]int64, len(subs))
		for i, sub := range subs {
			subsChatIDs[i] = sub.ChatID
		}

		updates, err := poller.linkState.Fetch(ctx, link)
		if err != nil {
			poller.logger.Warn("can't fetch link state", "err", err)
			if errors.Is(err, context.DeadlineExceeded) {
				// no need to notify
				continue
			}
			poller.updateAndNotify(ctx, link, subsChatIDs, []model.LinkState{{
				UpdatedBy:  "-",
				UpdatedAt:  time.Now(),
				Title:      "Could not fetch link state",
				UpdateInfo: "Cause: " + err.Error(),
			}})
			continue
		}
		if updates == nil || len(updates) == 0 {
			continue
		}
		poller.updateAndNotify(ctx, link, subsChatIDs, updates)
	}
}

func (poller *LinkPollingUseCase) updateAndNotify(
	ctx context.Context, link domain.Link,
	subsChatIDs []int64, updates []model.LinkState,
) {
	youngestEvent := updates[0].UpdatedAt
	for _, update := range updates {
		if update.UpdatedAt.After(youngestEvent) {
			youngestEvent = update.UpdatedAt
		}
	}
	if !youngestEvent.After(link.LastUpdate) {
		return
	}
	// something new
	link.LastUpdate = youngestEvent
	updateEvents := make([]model.LinkUpdatedEvent, len(updates))
	for i, update := range updates {
		upd := model.LinkUpdatedEvent{
			ChatIDs:      subsChatIDs,
			UpdatedAt:    update.UpdatedAt,
			UpdateAuthor: update.UpdatedBy,
			UpdateName:   update.Title,
			LinkID:       link.ID,
			RawURL:       link.URL.String(),
			Description:  update.UpdateInfo,
		}
		updateEvents[i] = upd
	}
	_, errU := poller.subscriptionRepository.UpdateLink(ctx, link, updateEvents)
	if errU != nil {
		poller.logger.Error("can't update link state", "err", errU)
	}
}
