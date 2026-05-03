package httpx

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/chintamani/habit-tracker/internal/store"
)

type todayItem struct {
	Habit     store.Habit
	CheckedIn bool
	Streak    int
	Weekly    int
}

type todayPage struct {
	Today    time.Time
	Weekday  string // long form, e.g. "Saturday"
	Morning  []todayItem
	Anytime  []todayItem
	Evening  []todayItem
	Done     int // habits completed today
	Total    int // total active habits today
	Note     store.Note
	NoteForm string
}

func (s *Server) handleToday(w http.ResponseWriter, _ *http.Request) {
	today := s.today()
	habits, err := s.store.ListActiveHabits()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	page := todayPage{Today: today, Weekday: today.Weekday().String()}
	for _, h := range habits {
		item, err := s.buildItem(h, today)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		page.Total++
		if item.CheckedIn {
			page.Done++
		}
		switch h.Routine {
		case store.Morning:
			page.Morning = append(page.Morning, item)
		case store.Evening:
			page.Evening = append(page.Evening, item)
		default:
			page.Anytime = append(page.Anytime, item)
		}
	}
	note, err := s.store.GetNote(today)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	page.Note = note
	page.NoteForm = note.Body

	s.tmpls.renderPage(w, "today.html", page)
}

func (s *Server) buildItem(h store.Habit, today time.Time) (todayItem, error) {
	in, err := s.store.IsCheckedIn(h.ID, today)
	if err != nil {
		return todayItem{}, err
	}
	streak, err := s.store.CurrentStreak(h.ID, today)
	if err != nil {
		return todayItem{}, err
	}
	weekly := 0
	if h.Cadence == store.Weekly {
		weekly, err = s.store.WeeklyProgress(h.ID, today)
		if err != nil {
			return todayItem{}, err
		}
	}
	return todayItem{Habit: h, CheckedIn: in, Streak: streak, Weekly: weekly}, nil
}

func (s *Server) handleToggleCheckIn(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	id, err := strconv.ParseInt(r.FormValue("habit_id"), 10, 64)
	if err != nil {
		http.Error(w, "bad habit_id", 400)
		return
	}
	today := s.today()
	if _, err := s.store.ToggleCheckIn(id, today); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	h, err := s.store.GetHabit(id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	item, err := s.buildItem(h, today)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	s.tmpls.renderFragment(w, "habit_row", item)
}

type historyHabit struct {
	Habit store.Habit
	Days  []historyDay
}

type historyDay struct {
	Day     time.Time
	Checked bool
}

type historyPage struct {
	Days   []time.Time // 30 days, oldest first
	Habits []historyHabit
}

func (s *Server) handleHistory(w http.ResponseWriter, _ *http.Request) {
	today := s.today()
	from := today.AddDate(0, 0, -29)
	days := make([]time.Time, 30)
	for i := range 30 {
		days[i] = from.AddDate(0, 0, i)
	}

	habits, err := s.store.ListActiveHabits()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	page := historyPage{Days: days}
	for _, h := range habits {
		set, err := s.store.CheckedInDays(h.ID, from, today)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		hh := historyHabit{Habit: h, Days: make([]historyDay, 30)}
		for i, d := range days {
			hh.Days[i] = historyDay{Day: d, Checked: set[d.Format("2006-01-02")]}
		}
		page.Habits = append(page.Habits, hh)
	}
	s.tmpls.renderPage(w, "history.html", page)
}

type habitsPage struct {
	Habits []store.Habit
	Form   habitFormValues
	Errors map[string]string
}

type habitFormValues struct {
	ID          int64
	Name        string
	Routine     string
	Cadence     string
	TargetCount int
}

func (s *Server) handleListHabits(w http.ResponseWriter, _ *http.Request) {
	habits, err := s.store.ListAllHabits()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	s.tmpls.renderPage(w, "habits.html", habitsPage{
		Habits: habits,
		Form:   habitFormValues{Routine: "anytime", Cadence: "daily", TargetCount: 1},
	})
}

func (s *Server) handleCreateHabit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	form := habitFormValues{
		Name:        r.FormValue("name"),
		Routine:     r.FormValue("routine"),
		Cadence:     r.FormValue("cadence"),
		TargetCount: atoi(r.FormValue("target_count"), 1),
	}

	errs := validateHabitForm(form)
	if len(errs) > 0 {
		habits, _ := s.store.ListAllHabits()
		s.tmpls.renderPage(w, "habits.html", habitsPage{Habits: habits, Form: form, Errors: errs})
		return
	}
	r1, _ := store.ParseRoutine(form.Routine)
	c1, _ := store.ParseCadence(form.Cadence)
	if _, err := s.store.CreateHabit(form.Name, r1, c1, form.TargetCount); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/habits", http.StatusSeeOther)
}

func (s *Server) handleEditHabit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", 400)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	form := habitFormValues{
		ID:          id,
		Name:        r.FormValue("name"),
		Routine:     r.FormValue("routine"),
		Cadence:     r.FormValue("cadence"),
		TargetCount: atoi(r.FormValue("target_count"), 1),
	}
	if errs := validateHabitForm(form); len(errs) > 0 {
		habits, _ := s.store.ListAllHabits()
		s.tmpls.renderPage(w, "habits.html", habitsPage{Habits: habits, Form: form, Errors: errs})
		return
	}
	r1, _ := store.ParseRoutine(form.Routine)
	c1, _ := store.ParseCadence(form.Cadence)
	if _, err := s.store.UpdateHabit(id, form.Name, r1, c1, form.TargetCount); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/habits", http.StatusSeeOther)
}

func (s *Server) handleArchiveHabit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", 400)
		return
	}
	if err := s.store.ArchiveHabit(id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/habits", http.StatusSeeOther)
}

type notePage struct {
	Day  time.Time
	Note store.Note
}

func (s *Server) handleGetNote(w http.ResponseWriter, r *http.Request) {
	day, err := parseDay(chi.URLParam(r, "day"), s.loc)
	if err != nil {
		http.Error(w, "bad day", 400)
		return
	}
	note, err := s.store.GetNote(day)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	s.tmpls.renderPage(w, "note.html", notePage{Day: day, Note: note})
}

func (s *Server) handleSaveNote(w http.ResponseWriter, r *http.Request) {
	day, err := parseDay(chi.URLParam(r, "day"), s.loc)
	if err != nil {
		http.Error(w, "bad day", 400)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err := s.store.UpsertNote(day, r.FormValue("body")); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if isHTMX(r) {
		w.Write([]byte(`<span class="saved">saved</span>`))
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func validateHabitForm(f habitFormValues) map[string]string {
	errs := map[string]string{}
	if f.Name == "" {
		errs["name"] = "name is required"
	}
	if _, err := store.ParseRoutine(f.Routine); err != nil {
		errs["routine"] = err.Error()
	}
	if _, err := store.ParseCadence(f.Cadence); err != nil {
		errs["cadence"] = err.Error()
	}
	if f.TargetCount < 1 {
		errs["target_count"] = "must be ≥ 1"
	}
	return errs
}

func parseDay(s string, loc *time.Location) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", s, loc)
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func atoi(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
