package store

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCreateAndGetHabit(t *testing.T) {
	s := newTestStore(t)
	h, err := s.CreateHabit("Read 30 min", Morning, Daily, 1)
	if err != nil {
		t.Fatal(err)
	}
	if h.ID == 0 || h.Name != "Read 30 min" || h.Routine != Morning {
		t.Fatalf("unexpected habit: %+v", h)
	}

	got, err := s.GetHabit(h.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != h.Name {
		t.Fatalf("get mismatch: %+v", got)
	}
}

func TestListActiveExcludesArchived(t *testing.T) {
	s := newTestStore(t)
	a, _ := s.CreateHabit("A", Morning, Daily, 1)
	b, _ := s.CreateHabit("B", Evening, Daily, 1)
	if err := s.ArchiveHabit(b.ID); err != nil {
		t.Fatal(err)
	}
	active, err := s.ListActiveHabits()
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].ID != a.ID {
		t.Fatalf("expected only A in active, got %+v", active)
	}
}

func TestToggleCheckInIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	h, _ := s.CreateHabit("X", Anytime, Daily, 1)
	today := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)

	on, err := s.ToggleCheckIn(h.ID, today)
	if err != nil || !on {
		t.Fatalf("first toggle: on=%v err=%v", on, err)
	}
	in, _ := s.IsCheckedIn(h.ID, today)
	if !in {
		t.Fatal("expected checked-in after first toggle")
	}

	off, err := s.ToggleCheckIn(h.ID, today)
	if err != nil || off {
		t.Fatalf("second toggle: on=%v err=%v", off, err)
	}
	in, _ = s.IsCheckedIn(h.ID, today)
	if in {
		t.Fatal("expected NOT checked-in after second toggle")
	}
}

func TestCurrentStreak(t *testing.T) {
	s := newTestStore(t)
	h, _ := s.CreateHabit("Streaky", Anytime, Daily, 1)
	today := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)

	// Check in today, yesterday, day before. Skip 4 days ago.
	for _, off := range []int{0, -1, -2} {
		if _, err := s.ToggleCheckIn(h.ID, today.AddDate(0, 0, off)); err != nil {
			t.Fatal(err)
		}
	}
	// 4 days ago is a gap. Insert one before that to confirm streak stops.
	if _, err := s.ToggleCheckIn(h.ID, today.AddDate(0, 0, -5)); err != nil {
		t.Fatal(err)
	}

	streak, err := s.CurrentStreak(h.ID, today)
	if err != nil {
		t.Fatal(err)
	}
	if streak != 3 {
		t.Fatalf("expected streak=3, got %d", streak)
	}
}

func TestStreakAllowsMissingToday(t *testing.T) {
	s := newTestStore(t)
	h, _ := s.CreateHabit("Y", Anytime, Daily, 1)
	today := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	// Yesterday and day before only — today not yet logged.
	for _, off := range []int{-1, -2} {
		s.ToggleCheckIn(h.ID, today.AddDate(0, 0, off))
	}
	streak, err := s.CurrentStreak(h.ID, today)
	if err != nil {
		t.Fatal(err)
	}
	if streak != 2 {
		t.Fatalf("expected streak=2 even though today is missing, got %d", streak)
	}
}

func TestWeeklyProgress(t *testing.T) {
	s := newTestStore(t)
	h, _ := s.CreateHabit("Gym", Anytime, Weekly, 3)
	// Saturday in May 2026 — pick a fixed week.
	tue := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC) // Tuesday
	s.ToggleCheckIn(h.ID, tue)
	s.ToggleCheckIn(h.ID, tue.AddDate(0, 0, 1)) // Wed
	// Previous Sunday should NOT count toward this week.
	s.ToggleCheckIn(h.ID, tue.AddDate(0, 0, -2))
	n, err := s.WeeklyProgress(h.ID, tue)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected weekly=2, got %d", n)
	}
}

func TestUpsertNote(t *testing.T) {
	s := newTestStore(t)
	day := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)

	if err := s.UpsertNote(day, "rough morning"); err != nil {
		t.Fatal(err)
	}
	n, err := s.GetNote(day)
	if err != nil {
		t.Fatal(err)
	}
	if n.Body != "rough morning" {
		t.Fatalf("unexpected body: %q", n.Body)
	}

	if err := s.UpsertNote(day, "actually fine"); err != nil {
		t.Fatal(err)
	}
	n, _ = s.GetNote(day)
	if n.Body != "actually fine" {
		t.Fatalf("upsert did not update: %q", n.Body)
	}

	if err := s.UpsertNote(day, ""); err != nil {
		t.Fatal(err)
	}
	n, _ = s.GetNote(day)
	if n.Body != "" {
		t.Fatalf("empty body should clear note, got %q", n.Body)
	}
}

func TestCheckedInDays(t *testing.T) {
	s := newTestStore(t)
	h, _ := s.CreateHabit("H", Anytime, Daily, 1)
	today := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	s.ToggleCheckIn(h.ID, today)
	s.ToggleCheckIn(h.ID, today.AddDate(0, 0, -2))

	days, err := s.CheckedInDays(h.ID, today.AddDate(0, 0, -3), today)
	if err != nil {
		t.Fatal(err)
	}
	if !days["2026-05-02"] || !days["2026-04-30"] {
		t.Fatalf("missing expected days: %+v", days)
	}
	if days["2026-05-01"] {
		t.Fatalf("did not expect 2026-05-01 in set: %+v", days)
	}
}
