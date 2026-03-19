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
}

func NewLinkPollingUseCase(
	logger *slog.Logger, sched gocron.Scheduler,
	linkState ports.LinkStatePort, botPort ports.BotPort,
	subscriptionRepo repository.SubscriptionRepository,
	chatRepo repository.ChatRepository,
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
	}
}

func (poller *LinkPollingUseCase) Start(ctx context.Context) {
	_, err := poller.sched.NewJob(
		gocron.DurationJob(10*time.Minute),
		gocron.NewTask(func() {
			poller.logger.Info("Start polling job")
			links, err := poller.subscriptionRepository.GetAllLinks(ctx)
			if err != nil {
				// for now always nil
				return
			}
			for _, link := range links {
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

					lstate, err := poller.linkState.Fetch(ctx, link)
					if err != nil {
						poller.logger.Warn("can't fetch link state", "err", err)
					}
					poller.updateAndNotify(ctx, link, subsChatIDs, lstate)
				}(link)
			}
		}),
	)
	if err != nil {
		poller.logger.Error("can't create job", "err", err)
		return
	}
	poller.sched.Start()
}

func (poller *LinkPollingUseCase) updateAndNotify(
	ctx context.Context, link domain.Link,
	subsChatIDs []int64, linkState model.LinkState,
) {
	if link.LastUpdate.Before(linkState.UpdatedAt) {
		// something new
		link.LastUpdate = linkState.UpdatedAt
		_, errU := poller.subscriptionRepository.UpdateLink(ctx, link)
		errN := poller.botPort.NotifyAboutUpdatedLinks(
			ctx, model.LinkUpdatedEvent{
				ChatIDs:     subsChatIDs,
				LinkID:      link.ID,
				RawURL:      link.URL.String(),
				Description: "something new",
			},
		)
		if errU != nil {
			poller.logger.Error("can't update link state", "err", errU)
		}
		if errN != nil {
			poller.logger.Error("can't notify about update", "err", errN)
		}
	}
}
