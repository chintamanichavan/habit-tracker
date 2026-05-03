package store

import (
	"time"
)

const dateFmt = "2006-01-02"

type CheckIn struct {
	HabitID int64
	Day     time.Time
	Count   int
}

// ToggleCheckIn flips the check-in for (habit, day): if absent, inserts with count=1; if present, deletes.
// Returns true if the habit is now checked-in for that day.
func (s *Store) ToggleCheckIn(habitID int64, day time.Time) (bool, error) {
	d := day.Format(dateFmt)
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var existing int
	err = tx.QueryRow(`SELECT count FROM check_ins WHERE habit_id = ? AND day = ?`, habitID, d).Scan(&existing)
	switch err {
	case nil:
		if _, err := tx.Exec(`DELETE FROM check_ins WHERE habit_id = ? AND day = ?`, habitID, d); err != nil {
			return false, err
		}
		return false, tx.Commit()
	default:
		// includes sql.ErrNoRows: insert
		if _, err := tx.Exec(`INSERT INTO check_ins (habit_id, day, count) VALUES (?, ?, 1)`, habitID, d); err != nil {
			return false, err
		}
		return true, tx.Commit()
	}
}

// CheckedInDays returns the set of days (yyyy-mm-dd) the habit was checked in, within [from, to] inclusive.
func (s *Store) CheckedInDays(habitID int64, from, to time.Time) (map[string]bool, error) {
	rows, err := s.db.Query(
		`SELECT day FROM check_ins WHERE habit_id = ? AND day BETWEEN ? AND ?`,
		habitID, from.Format(dateFmt), to.Format(dateFmt),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]bool)
	for rows.Next() {
		var day string
		if err := rows.Scan(&day); err != nil {
			return nil, err
		}
		// Normalize: SQLite stores as 'YYYY-MM-DD HH:MM:SS' or 'YYYY-MM-DD' depending on driver path.
		if len(day) > 10 {
			day = day[:10]
		}
		out[day] = true
	}
	return out, rows.Err()
}

// IsCheckedIn returns whether (habit, day) has a check-in row.
func (s *Store) IsCheckedIn(habitID int64, day time.Time) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT 1 FROM check_ins WHERE habit_id = ? AND day = ?`, habitID, day.Format(dateFmt)).Scan(&n)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CurrentStreak returns the number of consecutive days up to and including `today` the habit was checked in.
// For weekly-cadence habits, callers should use WeeklyProgress instead.
func (s *Store) CurrentStreak(habitID int64, today time.Time) (int, error) {
	// Walk back day-by-day until a miss.
	rows, err := s.db.Query(
		`SELECT day FROM check_ins WHERE habit_id = ? AND day <= ? ORDER BY day DESC LIMIT 365`,
		habitID, today.Format(dateFmt),
	)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	streak := 0
	expected := today
	for rows.Next() {
		var day string
		if err := rows.Scan(&day); err != nil {
			return 0, err
		}
		if len(day) > 10 {
			day = day[:10]
		}
		if day == expected.Format(dateFmt) {
			streak++
			expected = expected.AddDate(0, 0, -1)
			continue
		}
		// First gap: stop. But allow today to be missing — only count from the most recent checked day.
		if streak == 0 && day == today.AddDate(0, 0, -1).Format(dateFmt) {
			streak++
			expected = today.AddDate(0, 0, -2)
			continue
		}
		break
	}
	return streak, rows.Err()
}

// WeeklyProgress returns count of check-ins in the ISO week containing `today`.
func (s *Store) WeeklyProgress(habitID int64, today time.Time) (int, error) {
	// Week starts Monday. Compute Monday of this week.
	weekday := int(today.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday → 7
	}
	monday := today.AddDate(0, 0, -(weekday - 1))
	sunday := monday.AddDate(0, 0, 6)

	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM check_ins WHERE habit_id = ? AND day BETWEEN ? AND ?`,
		habitID, monday.Format(dateFmt), sunday.Format(dateFmt),
	).Scan(&n)
	return n, err
}
