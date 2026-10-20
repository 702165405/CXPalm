package store

import (
	"encoding/json"
	"errors"
)

type NotificationPreference struct {
	Enabled    bool   `json:"enabled"`
	Generation string `json:"generation"`
}

func (s *Store) NotificationPreference(scope, thread string) (NotificationPreference, error) {
	var p NotificationPreference
	var value string
	err := s.db.QueryRow("SELECT COALESCE((SELECT value FROM settings WHERE key=?),'')", "notify:"+Hash(scope+"\n"+thread)).Scan(&value)
	if err != nil {
		return p, err
	}
	if value != "" {
		err = json.Unmarshal([]byte(value), &p)
	}
	return p, err
}

func (s *Store) SetNotificationPreference(scope, thread string, enabled bool) (NotificationPreference, error) {
	p := NotificationPreference{Enabled: enabled, Generation: Secret()}
	raw, _ := json.Marshal(p)
	_, err := s.db.Exec("INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", "notify:"+Hash(scope+"\n"+thread), string(raw))
	return p, err
}

type SteerMessage struct {
	Images []string `json:"images,omitempty"`
	ID     string   `json:"id"`
	Turn   string   `json:"turn"`
	Text   string   `json:"text"`
}

func (s *Store) SaveSteer(thread string, message SteerMessage) error {
	if message.ID == "" || message.Turn == "" {
		return errors.New("steer identity required")
	}
	raw, _ := json.Marshal(message)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT OR IGNORE INTO steer_messages(thread,id,payload) VALUES(?,?,?)", thread, message.ID, raw); err != nil {
		return err
	}
	// Presentation fallback only; never replay these records as native mutations.
	if _, err = tx.Exec("DELETE FROM steer_messages WHERE thread=? AND seq NOT IN (SELECT seq FROM steer_messages WHERE thread=? ORDER BY seq DESC LIMIT 256)", thread, thread); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SteerMessages(thread string) ([]SteerMessage, error) {
	rows, err := s.db.Query("SELECT payload FROM steer_messages WHERE thread=? ORDER BY seq", thread)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []SteerMessage{}
	for rows.Next() {
		var raw []byte
		var message SteerMessage
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &message); err != nil {
			return nil, err
		}
		result = append(result, message)
	}
	return result, rows.Err()
}
