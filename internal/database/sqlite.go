package database

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Database wraps a sql.DB connection.
type Database struct {
	db *sql.DB
}

// New opens a SQLite database at the given path and ensures the models table exists.
func New(path string) (*Database, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite: %w", err)
	}

	// Match the original bot's schema exactly, including the topic column and
	// the auto-incrementing primary key.
	schema := `
CREATE TABLE IF NOT EXISTS models (
    id INTEGER PRIMARY KEY,
    modelid INTEGER,
    modelname VARCHAR(75) NOT NULL,
    message TEXT NOT NULL,
    time INTEGER,
    channel VARCHAR(75),
    updatetime INTEGER,
    topic TEXT DEFAULT "",
    events INTEGER
);
`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating models table: %w", err)
	}

	return &Database{db: db}, nil
}

// Close closes the database connection.
func (d *Database) Close() error {
	return d.db.Close()
}

// AddModel inserts a new watched model.
func (d *Database) AddModel(modelid, modelname, message, channel string, update int, events bool) (int64, error) {
	eventsInt := 0
	if events {
		eventsInt = 1
	}
	result, err := d.db.Exec(
		"INSERT INTO models(modelid, modelname, message, time, channel, updatetime, events) VALUES (?,?,?,0,?,?,?)",
		modelid, modelname, message, channel, update, eventsInt,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// DeleteModel removes a watched model by modelid and channel.
func (d *Database) DeleteModel(modelid, channel string) (int64, error) {
	result, err := d.db.Exec(
		`DELETE FROM models WHERE modelid = ? AND channel = ?`,
		modelid, channel,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// UpdateModel updates an existing watched model's fields.
func (d *Database) UpdateModel(modelid, modelname, message, channel string, update int, events bool) (int64, error) {
	eventsInt := 0
	if events {
		eventsInt = 1
	}
	result, err := d.db.Exec(
		"UPDATE models SET modelname = ?, message = ?, updatetime = ?, events = ? WHERE modelid = ? AND channel = ?",
		modelname, message, update, eventsInt, modelid, channel,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetModels returns all watched models.
func (d *Database) GetModels() ([]Model, error) {
	rows, err := d.db.Query("SELECT * FROM models")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var models []Model
	for rows.Next() {
		var m Model
		var id int64
		var topic sql.NullString
		var timeNull sql.NullInt64
		var updatetime sql.NullInt64
		var eventsNull sql.NullInt64
		err := rows.Scan(&id, &m.ModelID, &m.ModelName, &m.Message, &timeNull, &m.Channel, &updatetime, &topic, &eventsNull)
		if err != nil {
			return nil, err
		}
		m.ID = id
		m.Time = timeNull.Int64
		m.Topic = normalizeTopic(topic.String)
		m.Update = int(updatetime.Int64)
		m.Events = eventsNull.Int64 != 0
		models = append(models, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return models, nil
}

// GetModel returns a single watched model by modelid and channel.
func (d *Database) GetModel(modelid, channel string) (*Model, error) {
	var m Model
	var id int64
	var topic sql.NullString
	var timeNull sql.NullInt64
	var updatetime sql.NullInt64
	var eventsNull sql.NullInt64
	err := d.db.QueryRow(
		"SELECT * FROM models WHERE modelid = ? AND channel = ?",
		modelid, channel,
	).Scan(&id, &m.ModelID, &m.ModelName, &m.Message, &timeNull, &m.Channel, &updatetime, &topic, &eventsNull)
	if err != nil {
		return nil, err
	}
	m.ID = id
	m.Time = timeNull.Int64
	m.Topic = normalizeTopic(topic.String)
	m.Update = int(updatetime.Int64)
	m.Events = eventsNull.Int64 != 0
	return &m, nil
}

// GetTopic returns the current topic for a model.
func (d *Database) GetTopic(modelid, channel string) (string, error) {
	var topic sql.NullString
	err := d.db.QueryRow(
		"SELECT topic FROM models WHERE modelid = ? AND channel = ?",
		modelid, channel,
	).Scan(&topic)
	if err != nil {
		return "", err
	}
	return topic.String, nil
}

// UpdateTopic updates the topic for a model.
func (d *Database) UpdateTopic(modelid, topic, channel string) (int64, error) {
	result, err := d.db.Exec(
		"UPDATE models SET topic = ? WHERE modelid = ? AND channel = ?",
		topic, modelid, channel,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// UpdateTime updates the time field for a model.
func (d *Database) UpdateTime(modelid string, timeVal int64, channel string) (int64, error) {
	result, err := d.db.Exec(
		"UPDATE models SET time = ? WHERE modelid = ? AND channel = ?",
		timeVal, modelid, channel,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Model represents a watched model in the database.
type Model struct {
	ID        int64
	ModelID   string
	ModelName string
	Message   string
	Time      int64 // unix milliseconds of the last public-show notification; 0 when offline
	Channel   string
	Topic     string
	Update    int
	Events    bool
}

// normalizeTopic treats SQLite's integer 0 (written by the old Node bot for
// "false") the same as an empty topic string.
func normalizeTopic(s string) string {
	if s == "0" {
		return ""
	}
	return s
}
