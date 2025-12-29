package mediagroup

import (
	"fmt"
	"sync"
	"time"
)

type Item struct {
	ChatID       int64
	UserID       int64
	Username     string
	MediaGroupID string
	Caption      string
	FileID       string
}

type Group struct {
	ChatID   int64
	UserID   int64
	Username string
	Caption  string
	FileIDs  []string
}

type Options struct {
	Debounce time.Duration
	OnFlush  func(Group)
}

type Aggregator struct {
	mu       sync.Mutex
	debounce time.Duration
	onFlush  func(Group)
	groups   map[string]*pendingGroup
}

type pendingGroup struct {
	group Group
	timer *time.Timer
}

func New(opts Options) *Aggregator {
	debounce := opts.Debounce
	if debounce <= 0 {
		debounce = 1200 * time.Millisecond
	}

	return &Aggregator{
		debounce: debounce,
		onFlush:  opts.OnFlush,
		groups:   make(map[string]*pendingGroup),
	}
}

func (a *Aggregator) Add(item Item) {
	if item.MediaGroupID == "" || item.FileID == "" {
		return
	}

	key := makeKey(item.ChatID, item.MediaGroupID)

	a.mu.Lock()
	defer a.mu.Unlock()

	pg, ok := a.groups[key]
	if !ok {
		pg = &pendingGroup{
			group: Group{
				ChatID:   item.ChatID,
				UserID:   item.UserID,
				Username: item.Username,
				Caption:  item.Caption,
				FileIDs:  []string{item.FileID},
			},
		}
		a.groups[key] = pg
	} else {
		pg.group.FileIDs = append(pg.group.FileIDs, item.FileID)
		if item.Caption != "" {
			pg.group.Caption = item.Caption
		}
	}

	if pg.timer != nil {
		pg.timer.Stop()
	}
	pg.timer = time.AfterFunc(a.debounce, func() {
		a.flush(key)
	})
}

func (a *Aggregator) flush(key string) {
	a.mu.Lock()
	pg, ok := a.groups[key]
	if !ok {
		a.mu.Unlock()
		return
	}
	delete(a.groups, key)
	group := pg.group
	onFlush := a.onFlush
	a.mu.Unlock()

	if onFlush != nil {
		onFlush(group)
	}
}

func makeKey(chatID int64, mediaGroupID string) string {
	return fmt.Sprintf("%d:%s", chatID, mediaGroupID)
}
