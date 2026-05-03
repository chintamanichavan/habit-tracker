package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Routine string

const (
	Morning Routine = "morning"
	Evening Routine = "evening"
	Anytime Routine = "anytime"
)

func ParseRoutine(s string) (Routine, error) {
	switch Routine(s) {
	case Morning, Evening, Anytime:
		return Routine(s), nil
	default:
		return "", fmt.Errorf("invalid routine %q", s)
	}
}

type Cadence string

const (
	Daily  Cadence = "daily"
	Weekly Cadence = "weekly"
)

func ParseCadence(s string) (Cadence, error) {
	switch Cadence(s) {
	case Daily, Weekly:
		return Cadence(s), nil
	default:
		return "", fmt.Errorf("invalid cadence %q", s)
	}
}

type Habit struct {
	ID          int64
	Name        string
	Routine     Routine
	Cadence     Cadence
	TargetCount int
	ArchivedAt  sql.NullTime
	CreatedAt   time.Time
}

func (h Habit) Archived() bool { return h.ArchivedAt.Valid }

func (s *Store) CreateHabit(name string, r Routine, c Cadence, target int) (Habit, error) {
	if name == "" {
		return Habit{}, errors.New("name required")
	}
	if target < 1 {
		target = 1
	}
	res, err := s.db.Exec(
		`INSERT INTO habits (name, routine, cadence, target_count) VALUES (?, ?, ?, ?)`,
		name, string(r), string(c), target,
	)
	if err != nil {
		return Habit{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetHabit(id)
}

func (s *Store) UpdateHabit(id int64, name string, r Routine, c Cadence, target int) (Habit, error) {
	if name == "" {
		return Habit{}, errors.New("name required")
	}
	if target < 1 {
		target = 1
	}
	if _, err := s.db.Exec(
		`UPDATE habits SET name = ?, routine = ?, cadence = ?, target_count = ? WHERE id = ?`,
		name, string(r), string(c), target, id,
	); err != nil {
		return Habit{}, err
	}
	return s.GetHabit(id)
}

func (s *Store) ArchiveHabit(id int64) error {
	_, err := s.db.Exec(`UPDATE habits SET archived_at = CURRENT_TIMESTAMP WHERE id = ? AND archived_at IS NULL`, id)
	return err
}

func (s *Store) GetHabit(id int64) (Habit, error) {
	row := s.db.QueryRow(`SELECT id, name, routine, cadence, target_count, archived_at, created_at FROM habits WHERE id = ?`, id)
	return scanHabit(row)
}

// ListActiveHabits returns habits not archived, grouped order matches the routine sequence.
func (s *Store) ListActiveHabits() ([]Habit, error) {
	rows, err := s.db.Query(`
		SELECT id, name, routine, cadence, target_count, archived_at, created_at
		FROM habits
		WHERE archived_at IS NULL
		ORDER BY CASE routine WHEN 'morning' THEN 0 WHEN 'anytime' THEN 1 WHEN 'evening' THEN 2 END, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanHabits(rows)
}

func (s *Store) ListAllHabits() ([]Habit, error) {
	rows, err := s.db.Query(`
		SELECT id, name, routine, cadence, target_count, archived_at, created_at
		FROM habits
		ORDER BY archived_at IS NULL DESC, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanHabits(rows)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanHabit(r rowScanner) (Habit, error) {
	var h Habit
	var routine, cadence string
	if err := r.Scan(&h.ID, &h.Name, &routine, &cadence, &h.TargetCount, &h.ArchivedAt, &h.CreatedAt); err != nil {
		return Habit{}, err
	}
	h.Routine = Routine(routine)
	h.Cadence = Cadence(cadence)
	return h, nil
}

func scanHabits(rows *sql.Rows) ([]Habit, error) {
	var hs []Habit
	for rows.Next() {
		h, err := scanHabit(rows)
		if err != nil {
			return nil, err
		}
		hs = append(hs, h)
	}
	return hs, rows.Err()
}
