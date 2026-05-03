package store

import (
	"database/sql"
	"time"
)

type Note struct {
	Day       time.Time
	Body      string
	UpdatedAt time.Time
}

func (s *Store) GetNote(day time.Time) (Note, error) {
	d := day.Format(dateFmt)
	row := s.db.QueryRow(`SELECT day, body, updated_at FROM notes WHERE day = ?`, d)
	var n Note
	if err := row.Scan(&n.Day, &n.Body, &n.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return Note{Day: day}, nil
		}
		return Note{}, err
	}
	return n, nil
}

func (s *Store) UpsertNote(day time.Time, body string) error {
	d := day.Format(dateFmt)
	if body == "" {
		_, err := s.db.Exec(`DELETE FROM notes WHERE day = ?`, d)
		return err
	}
	_, err := s.db.Exec(
		`INSERT INTO notes (day, body) VALUES (?, ?)
		 ON CONFLICT(day) DO UPDATE SET body = excluded.body, updated_at = CURRENT_TIMESTAMP`,
		d, body,
	)
	return err
}
