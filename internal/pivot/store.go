package pivot

import (
	"errors"
	"sync/atomic"
	"time"
)

type Period string

const (
	PeriodDaily  Period = "1d"
	PeriodWeekly Period = "1w"
)

type Snapshot struct {
	Period    Period            `json:"period"`
	UpdatedAt time.Time         `json:"updated_at"`
	Symbols   map[string]Levels `json:"symbols"`
}

type Store struct {
	daily  atomic.Value
	weekly atomic.Value
}

func NewStore() *Store {
	s := &Store{}
	s.daily.Store((*Snapshot)(nil))
	s.weekly.Store((*Snapshot)(nil))
	return s
}

func (s *Store) Snapshot(period Period) (*Snapshot, error) {
	switch period {
	case PeriodDaily:
		snap, _ := s.daily.Load().(*Snapshot)
		return snap, nil
	case PeriodWeekly:
		snap, _ := s.weekly.Load().(*Snapshot)
		return snap, nil
	default:
		return nil, errors.New("unknown period")
	}
}

func (s *Store) Swap(period Period, snap *Snapshot) error {
	if snap == nil {
		return errors.New("nil snapshot")
	}
	switch period {
	case PeriodDaily:
		s.daily.Store(snap)
		return nil
	case PeriodWeekly:
		s.weekly.Store(snap)
		return nil
	default:
		return errors.New("unknown period")
	}
}

func (s *Store) GetLevels(period Period, symbol string) (Levels, bool) {
	snap, err := s.Snapshot(period)
	if err != nil || snap == nil {
		return Levels{}, false
	}
	lv, ok := snap.Symbols[symbol]
	return lv, ok
}
