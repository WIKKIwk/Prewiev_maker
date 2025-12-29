package session

import (
	"sync"
	"time"
)

type HistoryMessage struct {
	Role      string
	Content   string
	ImageURLs []string
}

type Session struct {
	UserID       int64
	Username     string
	History      []HistoryMessage
	LastActivity time.Time
}

type Options struct {
	MaxMessages int
}

type Store struct {
	mu         sync.Mutex
	sessions   map[int64]*Session
	maxHistory int
}

func NewStore(opts Options) *Store {
	maxHistory := opts.MaxMessages
	if maxHistory <= 0 {
		maxHistory = 20
	}

	return &Store{
		sessions:   make(map[int64]*Session),
		maxHistory: maxHistory,
	}
}

func (s *Store) Clear(userID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sess, ok := s.sessions[userID]; ok {
		sess.History = nil
		sess.LastActivity = time.Now()
	}
}

func (s *Store) Snapshot(userID int64, username string) []HistoryMessage {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess := s.getOrCreateLocked(userID, username)
	sess.LastActivity = time.Now()

	history := make([]HistoryMessage, len(sess.History))
	copy(history, sess.History)
	return history
}

func (s *Store) Append(userID int64, username string, msgs ...HistoryMessage) {
	if len(msgs) == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	sess := s.getOrCreateLocked(userID, username)
	sess.LastActivity = time.Now()

	sess.History = append(sess.History, msgs...)
	if len(sess.History) > s.maxHistory {
		sess.History = sess.History[len(sess.History)-s.maxHistory:]
	}
}

func (s *Store) getOrCreateLocked(userID int64, username string) *Session {
	if sess, ok := s.sessions[userID]; ok {
		if sess.Username == "" && username != "" {
			sess.Username = username
		}
		return sess
	}

	sess := &Session{
		UserID:       userID,
		Username:     username,
		History:      nil,
		LastActivity: time.Now(),
	}
	s.sessions[userID] = sess
	return sess
}
