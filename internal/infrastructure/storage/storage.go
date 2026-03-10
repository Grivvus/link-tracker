package storage

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type Storage struct {
	linkStorage            map[int64]domain.Link
	subscriptionStorage    map[int64]domain.Subscription
	linkIDByURL            map[string]int64
	subscriptionIDByChatID map[int64]map[int64]int64
	chatStorage            map[int64]struct{}
	currentMaxLinkID       atomic.Int64
	currentMaxSubID        atomic.Int64
	rwmuChat               sync.RWMutex
	rwmuLink               sync.RWMutex
	rwmuSubscription       sync.RWMutex
}

var storage *Storage = nil

func NewSubscriptionStorage() repository.SubscriptionRepository {
	if storage == nil {
		storage = &Storage{
			linkStorage:            make(map[int64]domain.Link, 10),
			subscriptionStorage:    make(map[int64]domain.Subscription, 10),
			linkIDByURL:            make(map[string]int64, 10),
			subscriptionIDByChatID: make(map[int64]map[int64]int64, 10),
			chatStorage:            make(map[int64]struct{}, 10),
		}
	}
	return storage
}

func NewChatStorage() repository.ChatRepository {
	if storage == nil {
		storage = &Storage{
			linkStorage:            make(map[int64]domain.Link, 10),
			subscriptionStorage:    make(map[int64]domain.Subscription, 10),
			linkIDByURL:            make(map[string]int64, 10),
			subscriptionIDByChatID: make(map[int64]map[int64]int64, 10),
			chatStorage:            make(map[int64]struct{}, 10),
		}
	}
	return storage
}

func (st *Storage) Subscribe(
	ctx context.Context, link domain.Link, chatID int64, tags []string,
) (domain.Subscription, error) {
	st.rwmuChat.RLock()
	_, chatExists := st.chatStorage[chatID]
	st.rwmuChat.RUnlock()
	if !chatExists {
		return domain.Subscription{}, fmt.Errorf(
			"%w: no chat with this id", repository.ErrEntityNotFound,
		)
	}

	linkID, err := st.saveOrGetLink(link)
	if err != nil {
		return domain.Subscription{}, err
	}

	st.rwmuSubscription.Lock()
	defer st.rwmuSubscription.Unlock()

	if _, ok := st.subscriptionIDByChatID[chatID]; !ok {
		st.subscriptionIDByChatID[chatID] = make(map[int64]int64, 2)
	}
	if _, alreadySubscribed := st.subscriptionIDByChatID[chatID][linkID]; alreadySubscribed {
		return domain.Subscription{}, repository.ErrEntityAlreadyExists
	}

	subID := st.getNextSubscriptionID()
	newSub := domain.NewSubscription(subID, linkID, chatID, tags)
	st.subscriptionStorage[subID] = newSub
	st.subscriptionIDByChatID[chatID][linkID] = subID
	return newSub, nil
}

func (st *Storage) Unsubscribe(
	ctx context.Context, link domain.Link, chatID int64,
) error {
	st.rwmuChat.RLock()
	_, chatExists := st.chatStorage[chatID]
	st.rwmuChat.RUnlock()
	if !chatExists {
		return fmt.Errorf("%w: no chat with this id", repository.ErrEntityNotFound)
	}

	linkID, err := st.findLinkID(link)
	if err != nil {
		return err
	}

	st.rwmuSubscription.Lock()
	defer st.rwmuSubscription.Unlock()
	byLink, ok := st.subscriptionIDByChatID[chatID]
	if !ok {
		return fmt.Errorf("%w: no subscriptions for this chat", repository.ErrEntityNotFound)
	}
	subID, ok := byLink[linkID]
	if !ok {
		return fmt.Errorf("%w: subscription not found", repository.ErrEntityNotFound)
	}
	delete(byLink, linkID)
	if len(byLink) == 0 {
		delete(st.subscriptionIDByChatID, chatID)
	}
	delete(st.subscriptionStorage, subID)
	return nil
}

func (st *Storage) FindLinkByURL(
	ctx context.Context, inputURL *url.URL,
) (domain.Link, error) {
	if inputURL == nil {
		return domain.Link{}, fmt.Errorf(
			"%w: nil link url", repository.ErrEntityNotFound,
		)
	}
	key := normalizeURL(inputURL)

	st.rwmuLink.RLock()
	defer st.rwmuLink.RUnlock()
	linkID, ok := st.linkIDByURL[key]
	if !ok {
		return domain.Link{}, fmt.Errorf("%w: no such link", repository.ErrEntityNotFound)
	}
	link, ok := st.linkStorage[linkID]
	if !ok {
		return domain.Link{}, fmt.Errorf("%w: no such link", repository.ErrEntityNotFound)
	}
	return link, nil
}

func (st *Storage) AllChatSubscriptions(
	ctx context.Context, chatID int64,
) ([]domain.Subscription, error) {
	st.rwmuChat.RLock()
	_, chatExists := st.chatStorage[chatID]
	st.rwmuChat.RUnlock()
	if !chatExists {
		return nil, fmt.Errorf("%w: no chat with this id", repository.ErrEntityNotFound)
	}

	st.rwmuSubscription.RLock()
	defer st.rwmuSubscription.RUnlock()

	byLink, ok := st.subscriptionIDByChatID[chatID]
	if !ok {
		return []domain.Subscription{}, nil
	}
	ret := make([]domain.Subscription, 0, len(byLink))
	for _, subID := range byLink {
		sub, ok := st.subscriptionStorage[subID]
		if ok {
			ret = append(ret, sub)
		}
	}
	return ret, nil
}

func (st *Storage) AllLinkSubscriptions(
	ctx context.Context, link domain.Link,
) ([]domain.Subscription, error) {
	linkID, err := st.findLinkID(link)
	if err != nil {
		return nil, err
	}
	st.rwmuSubscription.RLock()
	defer st.rwmuSubscription.RUnlock()

	ret := make([]domain.Subscription, 0, 2)
	for _, sub := range st.subscriptionStorage {
		if sub.LinkID == linkID {
			ret = append(ret, sub)
		}
	}
	return ret, nil
}

func (st *Storage) saveOrGetLink(link domain.Link) (int64, error) {
	if link.URL == nil {
		return 0, fmt.Errorf("%w: nil link url", repository.ErrEntityNotFound)
	}
	key := normalizeURL(link.URL)

	st.rwmuLink.Lock()
	defer st.rwmuLink.Unlock()
	if existingID, ok := st.linkIDByURL[key]; ok {
		return existingID, nil
	}

	newID := st.getNextLinkID()
	link.ID = newID
	st.linkStorage[newID] = link
	st.linkIDByURL[key] = newID
	return newID, nil
}

func (st *Storage) findLinkID(link domain.Link) (int64, error) {
	if link.ID > 0 {
		st.rwmuLink.RLock()
		defer st.rwmuLink.RUnlock()
		if _, ok := st.linkStorage[link.ID]; ok {
			return link.ID, nil
		}
		return 0, fmt.Errorf("%w: no such link", repository.ErrEntityNotFound)
	}
	if link.URL == nil {
		return 0, fmt.Errorf("%w: no such link", repository.ErrEntityNotFound)
	}
	key := normalizeURL(link.URL)

	st.rwmuLink.RLock()
	defer st.rwmuLink.RUnlock()
	linkID, ok := st.linkIDByURL[key]
	if !ok {
		return 0, fmt.Errorf("%w: no such link", repository.ErrEntityNotFound)
	}
	return linkID, nil
}

func (st *Storage) getNextLinkID() int64 {
	newID := st.currentMaxLinkID.Add(1)
	return newID
}

func (st *Storage) getNextSubscriptionID() int64 {
	newID := st.currentMaxSubID.Add(1)
	return newID
}

func (st *Storage) Register(ctx context.Context, chatID int64) error {
	st.rwmuChat.Lock()
	defer st.rwmuChat.Unlock()
	if _, ok := st.chatStorage[chatID]; ok {
		return repository.ErrEntityAlreadyExists
	}
	st.chatStorage[chatID] = struct{}{}
	return nil
}

func (st *Storage) Delete(ctx context.Context, chatID int64) error {
	st.rwmuChat.Lock()
	defer st.rwmuChat.Unlock()
	if _, ok := st.chatStorage[chatID]; !ok {
		return repository.ErrEntityNotFound
	}
	delete(st.chatStorage, chatID)
	return nil
}

func (st *Storage) isChatRegistered(chatID int64) bool {
	st.rwmuChat.RLock()
	defer st.rwmuChat.RUnlock()
	_, ok := st.chatStorage[chatID]
	return ok
}

func normalizeURL(linkURL *url.URL) string {
	return linkURL.String()
}
