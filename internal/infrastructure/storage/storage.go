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

const initialCap = 10

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

func New() *Storage {
	return &Storage{
		linkStorage:            make(map[int64]domain.Link, initialCap),
		subscriptionStorage:    make(map[int64]domain.Subscription, initialCap),
		linkIDByURL:            make(map[string]int64, initialCap),
		subscriptionIDByChatID: make(map[int64]map[int64]int64, initialCap),
		chatStorage:            make(map[int64]struct{}, initialCap),
	}
}

func (st *Storage) Subscribe(
	_ context.Context, link domain.Link, chatID int64, tags []string,
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
		st.subscriptionIDByChatID[chatID] = make(map[int64]int64)
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
	_ context.Context, link domain.Link, chatID int64,
) (domain.Subscription, error) {
	st.rwmuChat.RLock()
	_, chatExists := st.chatStorage[chatID]
	st.rwmuChat.RUnlock()
	if !chatExists {
		return domain.Subscription{}, fmt.Errorf(
			"%w: no chat with this id", repository.ErrEntityNotFound,
		)
	}

	linkID, err := st.findLinkID(link)
	if err != nil {
		return domain.Subscription{}, err
	}

	st.rwmuSubscription.Lock()
	defer st.rwmuSubscription.Unlock()
	byLink, ok := st.subscriptionIDByChatID[chatID]
	if !ok {
		return domain.Subscription{}, fmt.Errorf(
			"%w: no subscriptions for this chat",
			repository.ErrEntityNotFound,
		)
	}
	subID, ok := byLink[linkID]
	if !ok {
		return domain.Subscription{}, fmt.Errorf(
			"%w: subscription not found", repository.ErrEntityNotFound,
		)
	}
	delete(byLink, linkID)
	if len(byLink) == 0 {
		delete(st.subscriptionIDByChatID, chatID)
	}
	sub := st.subscriptionStorage[subID]
	delete(st.subscriptionStorage, subID)
	return domain.Subscription{
		ID:     subID,
		ChatID: chatID,
		LinkID: linkID,
		Tags:   sub.Tags,
	}, nil
}

func (st *Storage) GetAllLinks(_ context.Context) ([]domain.Link, error) {
	st.rwmuLink.RLock()
	defer st.rwmuLink.RUnlock()
	links := make([]domain.Link, 0, len(st.linkStorage))
	for _, v := range st.linkStorage {
		links = append(links, v)
	}
	return links, nil
}

func (st *Storage) GetLink(_ context.Context, linkID int64) (domain.Link, error) {
	st.rwmuLink.RLock()
	defer st.rwmuLink.RUnlock()
	link, ok := st.linkStorage[linkID]
	if !ok {
		return link, repository.ErrEntityNotFound
	}
	return link, nil
}

func (st *Storage) UpdateLink(_ context.Context, link domain.Link) (domain.Link, error) {
	_, err := st.saveOrGetLink(link)
	if err != nil {
		return domain.Link{}, err
	}
	return link, nil
}

func (st *Storage) FindLinkByURL(
	_ context.Context, inputURL *url.URL,
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
	_ context.Context, chatID int64,
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
	_ context.Context, link domain.Link,
) ([]domain.Subscription, error) {
	linkID, err := st.findLinkID(link)
	if err != nil {
		return nil, err
	}
	st.rwmuSubscription.RLock()
	defer st.rwmuSubscription.RUnlock()

	ret := make([]domain.Subscription, 0, initialCap)
	for _, sub := range st.subscriptionStorage {
		if sub.LinkID == linkID {
			ret = append(ret, sub)
		}
	}
	return ret, nil
}

func (st *Storage) Register(_ context.Context, chatID int64) error {
	st.rwmuChat.Lock()
	defer st.rwmuChat.Unlock()
	if _, ok := st.chatStorage[chatID]; ok {
		return repository.ErrEntityAlreadyExists
	}
	st.chatStorage[chatID] = struct{}{}
	return nil
}

func (st *Storage) Delete(_ context.Context, chatID int64) error {
	st.rwmuChat.Lock()
	defer st.rwmuChat.Unlock()
	if _, ok := st.chatStorage[chatID]; !ok {
		return repository.ErrEntityNotFound
	}
	delete(st.chatStorage, chatID)
	return nil
}

func (st *Storage) saveOrGetLink(link domain.Link) (int64, error) {
	if link.URL == nil {
		return 0, fmt.Errorf(
			"%w: nil link url", repository.ErrEntityNotFound,
		)
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

func normalizeURL(linkURL *url.URL) string {
	return linkURL.String()
}
