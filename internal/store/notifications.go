package store

import "encoding/json"

type Notification struct {
	Subscription string `json:"subscription,omitempty"`
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Thread       string `json:"thread"`
	Title        string `json:"title"`
	Time         int64  `json:"time"`
}

func (s *Store) Notifications(scope string) ([]Notification, error) {
	rows, err := s.db.Query("SELECT payload FROM notification_events WHERE scope=? ORDER BY seq", scope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Notification{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var v Notification
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
func (s *Store) SaveNotification(scope string, event Notification) error {
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT OR IGNORE INTO notification_events(scope,id,payload) VALUES(?,?,?)", scope, event.ID, raw); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM notification_events WHERE scope=? AND seq NOT IN (SELECT seq FROM notification_events WHERE scope=? ORDER BY seq DESC LIMIT 256)", scope, scope); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) NotificationCheckpoints(scope string) (map[string]json.RawMessage, error) {
	rows, err := s.db.Query("SELECT thread,payload FROM notification_checkpoints WHERE scope=?", scope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]json.RawMessage{}
	for rows.Next() {
		var id string
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		result[id] = json.RawMessage(raw)
	}
	return result, rows.Err()
}
func (s *Store) SaveNotificationCheckpoint(scope, thread string, raw []byte) error {
	_, err := s.db.Exec("INSERT INTO notification_checkpoints(scope,thread,payload) VALUES(?,?,?) ON CONFLICT(scope,thread) DO UPDATE SET payload=excluded.payload", scope, thread, raw)
	return err
}
