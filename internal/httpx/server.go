package httpx

import (
	"io/fs"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/chintamani/habit-tracker/internal/store"
)

type storeIface interface {
	CreateHabit(name string, r store.Routine, c store.Cadence, target int) (store.Habit, error)
	UpdateHabit(id int64, name string, r store.Routine, c store.Cadence, target int) (store.Habit, error)
	ArchiveHabit(id int64) error
	GetHabit(id int64) (store.Habit, error)
	ListActiveHabits() ([]store.Habit, error)
	ListAllHabits() ([]store.Habit, error)

	ToggleCheckIn(habitID int64, day time.Time) (bool, error)
	IsCheckedIn(habitID int64, day time.Time) (bool, error)
	CheckedInDays(habitID int64, from, to time.Time) (map[string]bool, error)
	CurrentStreak(habitID int64, today time.Time) (int, error)
	WeeklyProgress(habitID int64, today time.Time) (int, error)

	GetNote(day time.Time) (store.Note, error)
	UpsertNote(day time.Time, body string) error
}

func NewServer(s storeIface, loc *time.Location, templatesFS, staticFS fs.FS) (http.Handler, error) {
	tmpls, err := loadTemplates(templatesFS)
	if err != nil {
		return nil, err
	}
	srv := &Server{store: s, loc: loc, tmpls: tmpls, staticDir: http.FS(staticFS)}

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})

	r.Get("/", srv.handleToday)
	r.Post("/check-ins", srv.handleToggleCheckIn)

	r.Get("/history", srv.handleHistory)

	r.Get("/habits", srv.handleListHabits)
	r.Post("/habits", srv.handleCreateHabit)
	r.Post("/habits/{id}/edit", srv.handleEditHabit)
	r.Post("/habits/{id}/archive", srv.handleArchiveHabit)

	r.Get("/notes/{day}", srv.handleGetNote)
	r.Post("/notes/{day}", srv.handleSaveNote)

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(srv.staticDir)))

	return r, nil
}

// today returns the local-zone "today" anchored at midnight, used as the day key.
func (s *Server) today() time.Time {
	now := time.Now().In(s.loc)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.loc)
}
