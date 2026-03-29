package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/ports"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/scrapper/clients"
)

type LinkPollingUseCase struct {
	sched                  gocron.Scheduler
	logger                 *slog.Logger
	linkState              ports.LinkStatePort
	botPort                ports.BotPort
	subscriptionRepository repository.SubscriptionRepository
	chatRepository         repository.ChatRepository
	ghClient               *clients.GitHubClient
	stackoverflowClient    *clients.StackoverflowClient
	pollerIntervalMinutes  int64
}

func NewLinkPollingUseCase(
	logger *slog.Logger, sched gocron.Scheduler,
	linkState ports.LinkStatePort, botPort ports.BotPort,
	subscriptionRepo repository.SubscriptionRepository,
	chatRepo repository.ChatRepository, pollerIntervalMinutes int64,
) LinkPollingUseCase {
	return LinkPollingUseCase{
		sched:                  sched,
		logger:                 logger,
		linkState:              linkState,
		botPort:                botPort,
		subscriptionRepository: subscriptionRepo,
		chatRepository:         chatRepo,
		ghClient:               clients.NewGitHubClient(logger),
		stackoverflowClient:    clients.NewStackoverflowClient(logger),
		pollerIntervalMinutes:  pollerIntervalMinutes,
	}
}

func (poller *LinkPollingUseCase) Start(ctx context.Context) error {
	_, err := poller.sched.NewJob(
		gocron.DurationJob(time.Duration(poller.pollerIntervalMinutes)*time.Minute),
		gocron.NewTask(func() {
			poller.logger.Info("Start polling job")
			links, errs := poller.subscriptionRepository.GetAllLinks(ctx)
			go func() {
				for err := range errs {
					poller.logger.Error("can't fetch links", "err", err)
				}
			}()
			for link := range links {
				poller.logger.Info("Fetch link", "url", link.URL)
				go func(link domain.Link) {
					subs, err := poller.subscriptionRepository.AllLinkSubscriptions(ctx, link)
					if err != nil {
						poller.logger.Warn("can't fetch trackings", "err", err)
						return
					}
					subsChatIDs := make([]int64, 0, len(subs))
					for _, sub := range subs {
						subsChatIDs = append(subsChatIDs, sub.ChatID)
					}

					updates, err := poller.linkState.Fetch(ctx, link)
					if err != nil {
						poller.logger.Warn("can't fetch link state", "err", err)
					}
					if updates == nil || len(updates) == 0 {
						return
					}
					poller.updateAndNotify(ctx, link, subsChatIDs, updates)
				}(link)
			}
		}),
	)
	if err != nil {
		return err
	}
	poller.sched.Start()
	return nil
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
	_, errU := poller.subscriptionRepository.UpdateLink(ctx, link)
	if errU != nil {
		poller.logger.Error("can't update link state", "err", errU)
	}
	for _, update := range updates {
		errN := poller.botPort.NotifyAboutUpdatedLinks(
			ctx, model.LinkUpdatedEvent{
				ChatIDs:     subsChatIDs,
				LinkID:      link.ID,
				RawURL:      link.URL.String(),
				Description: update.UpdateInfo,
			},
		)
		if errN != nil {
			poller.logger.Error("can't notify about update", "err", errN)
		}
	}
}
