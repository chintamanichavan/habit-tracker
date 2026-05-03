CREATE TABLE IF NOT EXISTS habits (
  id           INTEGER PRIMARY KEY,
  name         TEXT NOT NULL,
  routine      TEXT NOT NULL CHECK (routine IN ('morning','evening','anytime')),
  cadence      TEXT NOT NULL DEFAULT 'daily' CHECK (cadence IN ('daily','weekly')),
  target_count INTEGER NOT NULL DEFAULT 1,
  archived_at  TIMESTAMP,
  created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS check_ins (
  habit_id   INTEGER NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
  day        DATE NOT NULL,
  count      INTEGER NOT NULL DEFAULT 1,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (habit_id, day)
);

CREATE INDEX IF NOT EXISTS idx_check_ins_day ON check_ins(day);

CREATE TABLE IF NOT EXISTS notes (
  day        DATE PRIMARY KEY,
  body       TEXT NOT NULL,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
