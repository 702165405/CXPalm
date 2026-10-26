package store

import (
	"database/sql"
	"errors"
)

func (s *Store) ReadToken(thread string) (string, error) {
	var token string
	err := s.db.QueryRow("SELECT token FROM thread_reads WHERE thread=?", thread).Scan(&token)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return token, err
}
func (s *Store) ReadTokens() (map[string]string, error) {
	rows, err := s.db.Query("SELECT thread,token FROM thread_reads")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, token string
		if err := rows.Scan(&id, &token); err != nil {
			return nil, err
		}
		out[id] = token
	}
	return out, rows.Err()
}
func (s *Store) SetReadToken(thread, token string) error {
	if token == "" {
		_, err := s.db.Exec("DELETE FROM thread_reads WHERE thread=?", thread)
		return err
	}
	_, err := s.db.Exec("INSERT INTO thread_reads(thread,token) VALUES(?,?) ON CONFLICT(thread) DO UPDATE SET token=excluded.token", thread, token)
	return err
}
