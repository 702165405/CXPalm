package store

import (
	"database/sql"
	"time"
)

type DeviceEvent struct {
	Device string `json:"device"`
	Name   string `json:"name"`
	Action string `json:"action"`
	At     int64  `json:"at"`
	IP     string `json:"ip"`
}

func (s *Store) migrateDeviceMetadata() error {
	rows, err := s.db.Query("PRAGMA table_info(devices)")
	if err != nil {
		return err
	}
	columns := map[string]bool{}
	for rows.Next() {
		var cid, nn, pk int
		var name, kind string
		var def any
		if err = rows.Scan(&cid, &name, &kind, &nn, &def, &pk); err != nil {
			rows.Close()
			return err
		}
		columns[name] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, c := range []struct{ name, definition string }{{"last_ip", "TEXT NOT NULL DEFAULT ''"}, {"last_seen", "INTEGER NOT NULL DEFAULT 0"}} {
		if !columns[c.name] {
			if _, err = s.db.Exec("ALTER TABLE devices ADD COLUMN " + c.name + " " + c.definition); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *Store) TouchDevice(id, ip string) error {
	now := time.Now().Unix()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var name string
	err = tx.QueryRow("UPDATE devices SET last_ip=?,last_seen=? WHERE id=? AND revoked=0 AND expires>? AND (last_ip!=? OR last_seen<=?) RETURNING name", ip, now, id, now, ip, now-300).Scan(&name)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT INTO device_events(device,name,action,at,ip) VALUES(?,?,'connected',?,?)", id, name, now, ip); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) DeviceEvents() ([]DeviceEvent, error) {
	rows, err := s.db.Query(`SELECT device,name,action,at,ip FROM (
 SELECT id,device,name,action,at,ip FROM device_events
 UNION ALL
 SELECT 0,id,name,CASE WHEN revoked=1 THEN 'legacy_revoked' WHEN expires<=? THEN 'legacy_expired' ELSE 'legacy_paired' END,created,''
 FROM devices d WHERE NOT EXISTS(SELECT 1 FROM device_events e WHERE e.device=d.id AND e.action IN ('paired','login'))
 ) ORDER BY at DESC,id DESC,device DESC LIMIT 200`, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []DeviceEvent{}
	for rows.Next() {
		var event DeviceEvent
		if err = rows.Scan(&event.Device, &event.Name, &event.Action, &event.At, &event.IP); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
