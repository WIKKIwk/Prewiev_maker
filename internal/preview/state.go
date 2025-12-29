package preview

import (
	"sync"
	"time"
)

type UIState struct {
	Mode          string
	GridPreset    string
	VerticalCount string
	AspectRatio   string

	ProductType string
	VisualStyle string
	HumanUsage  bool
	Custom      string

	SelectedFrames    [9]bool
	LastSelectedOrder []int

	LastPhotoFileID string
	MessageID       int

	AwaitingPhoto  bool
	AwaitingCustom bool
	Menu           string // "main" | "category" | "style" | "frames"

	UpdatedAt time.Time
}

func (s *UIState) SyncSelection() {
	n := desiredCount(*s)
	if n == 9 {
		for i := 0; i < 9; i++ {
			s.SelectedFrames[i] = true
		}
		s.LastSelectedOrder = []int{0, 1, 2, 3, 4, 5, 6, 7, 8}
		return
	}

	s.SelectedFrames = ensureExactlyNSelected(s.SelectedFrames, n)
	s.LastSelectedOrder = selectedIndices(s.SelectedFrames)
}

func (s *UIState) ToggleFrame(idx int) {
	n := desiredCount(*s)
	if n == 9 {
		return
	}
	if idx < 0 {
		idx = 0
	}
	if idx > 8 {
		idx = 8
	}

	wasOn := s.SelectedFrames[idx]
	if wasOn {
		s.SelectedFrames[idx] = false
		s.LastSelectedOrder = removeIndex(s.LastSelectedOrder, idx)

		for countTrue(s.SelectedFrames) < n {
			for i := 0; i < 9; i++ {
				if !s.SelectedFrames[i] {
					s.SelectedFrames[i] = true
					s.LastSelectedOrder = append(s.LastSelectedOrder, i)
					break
				}
			}
		}
		return
	}

	s.SelectedFrames[idx] = true
	s.LastSelectedOrder = removeIndex(s.LastSelectedOrder, idx)
	s.LastSelectedOrder = append(s.LastSelectedOrder, idx)

	for countTrue(s.SelectedFrames) > n && len(s.LastSelectedOrder) > 0 {
		oldest := s.LastSelectedOrder[0]
		s.LastSelectedOrder = s.LastSelectedOrder[1:]
		if oldest == idx {
			continue
		}
		s.SelectedFrames[oldest] = false
	}
	s.LastSelectedOrder = filterSelected(s.LastSelectedOrder, s.SelectedFrames)
}

func (s UIState) SelectionFrameIDs() []string {
	indices := selectionIndicesForOutput(s)
	out := make([]string, 0, len(indices))
	for _, idx := range indices {
		if idx < 0 || idx >= len(frameTemplates) {
			continue
		}
		out = append(out, frameTemplates[idx].ID)
	}
	return out
}

func (s UIState) PromptOptions() Options {
	return Options{
		Mode:          s.Mode,
		GridPreset:    s.GridPreset,
		VerticalCount: s.VerticalCount,
		AspectRatio:   s.AspectRatio,
		FrameIDs:      s.SelectionFrameIDs(),
		ProductType:   s.ProductType,
		VisualStyle:   s.VisualStyle,
		HumanUsage:    s.HumanUsage,
		Custom:        s.Custom,
	}
}

type Store struct {
	mu sync.Mutex
	m  map[stateKey]*UIState
}

type stateKey struct {
	ChatID int64
	UserID int64
}

func NewStore() *Store {
	return &Store{m: make(map[stateKey]*UIState)}
}

func (s *Store) Get(chatID, userID int64) UIState {
	s.mu.Lock()
	defer s.mu.Unlock()

	st := s.getOrCreateLocked(chatID, userID)
	st.SyncSelection()
	return *st
}

func (s *Store) Update(chatID, userID int64, fn func(*UIState)) UIState {
	s.mu.Lock()
	defer s.mu.Unlock()

	st := s.getOrCreateLocked(chatID, userID)
	if fn != nil {
		fn(st)
	}
	st.SyncSelection()
	st.UpdatedAt = time.Now()
	return *st
}

func (s *Store) Reset(chatID, userID int64) UIState {
	return s.Update(chatID, userID, func(st *UIState) {
		*st = defaultState()
	})
}

func (s *Store) getOrCreateLocked(chatID, userID int64) *UIState {
	key := stateKey{ChatID: chatID, UserID: userID}
	if st, ok := s.m[key]; ok {
		return st
	}
	st := defaultState()
	s.m[key] = &st
	return s.m[key]
}

func defaultState() UIState {
	var selected [9]bool
	for i := 0; i < 9; i++ {
		selected[i] = true
	}
	return UIState{
		Mode:              "grid",
		GridPreset:        "3x3",
		VerticalCount:     "4",
		AspectRatio:       "",
		ProductType:       "",
		VisualStyle:       "",
		HumanUsage:        false,
		Custom:            "",
		SelectedFrames:    selected,
		LastSelectedOrder: []int{0, 1, 2, 3, 4, 5, 6, 7, 8},
		Menu:              "main",
		UpdatedAt:         time.Now(),
	}
}

func desiredCount(st UIState) int {
	out := ResolveOutputPreset(Options{
		Mode:          st.Mode,
		GridPreset:    st.GridPreset,
		VerticalCount: st.VerticalCount,
		AspectRatio:   st.AspectRatio,
	})
	if out.Count < 1 {
		return 1
	}
	if out.Count > 9 {
		return 9
	}
	return out.Count
}

func selectionIndicesForOutput(st UIState) []int {
	s := st
	s.SyncSelection()
	n := desiredCount(s)

	idx := append([]int(nil), s.LastSelectedOrder...)
	if len(idx) > n {
		idx = idx[:n]
	}

	for len(idx) < n {
		added := false
		for i := 0; i < 9; i++ {
			if containsInt(idx, i) {
				continue
			}
			if s.SelectedFrames[i] {
				idx = append(idx, i)
				added = true
				break
			}
		}
		if added {
			continue
		}
		for i := 0; i < 9; i++ {
			if containsInt(idx, i) {
				continue
			}
			idx = append(idx, i)
			break
		}
	}

	if len(idx) > n {
		idx = idx[:n]
	}
	return idx
}

func ensureExactlyNSelected(selected [9]bool, n int) [9]bool {
	if n < 1 {
		n = 1
	}
	if n > 9 {
		n = 9
	}

	for countTrue(selected) > n {
		for i := 8; i >= 0; i-- {
			if selected[i] {
				selected[i] = false
				break
			}
		}
	}

	for countTrue(selected) < n {
		for i := 0; i < 9; i++ {
			if !selected[i] {
				selected[i] = true
				break
			}
		}
	}

	return selected
}

func selectedIndices(selected [9]bool) []int {
	out := make([]int, 0, 9)
	for i := 0; i < 9; i++ {
		if selected[i] {
			out = append(out, i)
		}
	}
	return out
}

func countTrue(selected [9]bool) int {
	n := 0
	for i := 0; i < 9; i++ {
		if selected[i] {
			n++
		}
	}
	return n
}

func removeIndex(list []int, idx int) []int {
	out := make([]int, 0, len(list))
	for _, v := range list {
		if v == idx {
			continue
		}
		out = append(out, v)
	}
	return out
}

func filterSelected(list []int, selected [9]bool) []int {
	out := make([]int, 0, len(list))
	for _, v := range list {
		if v < 0 || v >= 9 {
			continue
		}
		if !selected[v] {
			continue
		}
		out = append(out, v)
	}
	return out
}

func containsInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
