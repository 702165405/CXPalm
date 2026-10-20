package store

import (
	"encoding/json"
	"errors"
)

type QueuedMessage struct {
	Attachments []string `json:"attachments,omitempty"`
	ID          string   `json:"id"`
	Text        string   `json:"text"`
	Status      string   `json:"status"`
	Error       string   `json:"error,omitempty"`
	Device      string   `json:"-"`
	Skills      []string `json:"-"`
}

func (s *Store) Queue(thread string) ([]QueuedMessage, error) {
	rows, err := s.db.Query("SELECT id,device,text,skills,status,error,COALESCE((SELECT ids FROM queued_attachments WHERE message=id),'[]') FROM queued_messages WHERE thread=? ORDER BY seq", thread)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []QueuedMessage{}
	for rows.Next() {
		var q QueuedMessage
		var skills []byte
		var attachmentIDs []byte
		if err = rows.Scan(&q.ID, &q.Device, &q.Text, &skills, &q.Status, &q.Error, &attachmentIDs); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(skills, &q.Skills); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(attachmentIDs, &q.Attachments); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}
func (s *Store) QueueThreads() ([]string, error) {
	rows, err := s.db.Query("SELECT DISTINCT thread FROM queued_messages")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
func (s *Store) Enqueue(device, id, thread, text string, skills []string) (string, error) {
	return s.EnqueueFiles(device, id, thread, text, skills, nil)
}
func (s *Store) EnqueueFiles(device, id, thread, text string, skills, attachmentIDs []string) (string, error) {
	if !s.Valid(device) {
		return "", ErrUnauthorized
	}
	if (len(text) == 0 && len(attachmentIDs) == 0) || len(text) > 100000 || len(skills) > 8 || len(attachmentIDs) > 8 {
		return "", errors.New("invalid queued message")
	}
	if skills == nil {
		skills = []string{}
	}
	raw, err := json.Marshal(skills)
	if err != nil {
		return "", err
	}
	id = Hash(device + "\n" + id)
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`INSERT INTO queued_messages(id,device,thread,text,skills) SELECT ?,?,?,?,? WHERE (SELECT COUNT(*) FROM queued_messages WHERE thread=?)<16`, id, device, thread, text, raw, thread)
	if err != nil {
		return "", err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		err = errors.New("排队已满，最多保留 16 条消息")
	}
	if err != nil {
		return "", err
	}
	if len(attachmentIDs) > 0 {
		raw, _ := json.Marshal(attachmentIDs)
		if _, err = tx.Exec("INSERT INTO queued_attachments(message,ids) VALUES(?,?)", id, raw); err != nil {
			return "", err
		}
	}
	return id, tx.Commit()
}
func (s *Store) ClaimQueued(id string) (bool, error) {
	// Commit before IPC. A crash or ambiguous response can never replay this write.
	r, err := s.db.Exec("UPDATE queued_messages SET status='sending' WHERE id=? AND status='queued' AND NOT EXISTS(SELECT 1 FROM queued_messages head WHERE head.thread=queued_messages.thread AND head.seq<queued_messages.seq)", id)
	if err != nil {
		return false, err
	}
	n, err := r.RowsAffected()
	return n == 1, err
}
func (s *Store) FinishQueued(id, status, message string) error {
	if status != "sent" && status != "unknown" {
		return errors.New("invalid queue result")
	}
	_, err := s.db.Exec("UPDATE queued_messages SET status=?,error=? WHERE id=? AND status='sending'", status, message, id)
	return err
}
func (s *Store) CancelQueued(thread, id string) error {
	r, err := s.db.Exec("DELETE FROM queued_messages WHERE thread=? AND id=? AND status IN ('queued','unknown','sent')", thread, id)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err == nil && n == 0 {
		err = errors.New("消息已经开始发送或不在队列中，请核对桌面")
	}
	return err
}
func (s *Store) ObserveQueued(id string) error {
	_, err := s.db.Exec("DELETE FROM queued_messages WHERE id=? AND status='sent'", id)
	return err
}

func (s *Store) QueueAuthorized(device string) bool {
	if !s.Valid(device) {
		return false
	}
	var ip string
	if err := s.db.QueryRow("SELECT last_ip FROM devices WHERE id=?", device).Scan(&ip); err != nil {
		return false
	}
	return s.IPAllowed(ip)
}
