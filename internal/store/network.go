package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/netip"
	"sort"
	"strings"
	"time"
)

type IPPolicy struct {
	Enabled bool     `json:"enabled"`
	Entries []string `json:"entries"`
}

func NormalizeIPPolicy(p IPPolicy) (IPPolicy, error) {
	if len(p.Entries) > 64 {
		return p, errors.New("IP whitelist supports at most 64 entries")
	}
	entries := []string{}
	seen := map[string]bool{}
	for _, raw := range p.Entries {
		value := strings.TrimSpace(raw)
		if value == "" || len(value) > 80 {
			return p, errors.New("Invalid IP or CIDR entry")
		}
		if strings.Contains(value, "/") {
			prefix, err := netip.ParsePrefix(value)
			if err != nil || prefix.Addr().Zone() != "" || prefix.Addr().Is4In6() || prefix.Bits() == 0 {
				return p, errors.New("Use an explicit IPv4/IPv6 address or bounded CIDR, not a hostname or all-address range")
			}
			value = prefix.Masked().String()
		} else {
			address, err := netip.ParseAddr(value)
			if err != nil || address.Zone() != "" || address.IsUnspecified() || address.IsMulticast() {
				return p, errors.New("Invalid IP address")
			}
			value = address.Unmap().String()
		}
		if !seen[value] {
			seen[value] = true
			entries = append(entries, value)
		}
	}
	if p.Enabled && len(entries) == 0 {
		return p, errors.New("An enabled IP whitelist must contain an address or CIDR")
	}
	sort.Strings(entries)
	p.Entries = entries
	return p, nil
}
func (s *Store) IPPolicy() (IPPolicy, error) {
	var raw string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key='ip-policy'").Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return IPPolicy{Entries: []string{}}, nil
		}
		return IPPolicy{}, err
	}
	var p IPPolicy
	if err = json.Unmarshal([]byte(raw), &p); err != nil {
		return p, err
	}
	return NormalizeIPPolicy(p)
}
func (s *Store) SetIPPolicy(p IPPolicy) (IPPolicy, error) {
	p, err := NormalizeIPPolicy(p)
	if err != nil {
		return p, err
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return p, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return p, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT INTO settings(key,value) VALUES('ip-policy',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", string(raw)); err != nil {
		return p, err
	}
	action := "ip_policy_disabled"
	if p.Enabled {
		action = "ip_policy_enabled"
	}
	if _, err = tx.Exec("INSERT INTO device_events(device,name,action,at,ip) VALUES('','IP whitelist',?,?,'')", action, time.Now().Unix()); err != nil {
		return p, err
	}
	return p, tx.Commit()
}
func (s *Store) IPAllowed(value string) bool {
	p, err := s.IPPolicy()
	if err != nil {
		return false
	}
	if !p.Enabled {
		return true
	}
	address, err := netip.ParseAddr(value)
	if err != nil || address.Zone() != "" {
		return false
	}
	address = address.Unmap()
	for _, entry := range p.Entries {
		if strings.Contains(entry, "/") {
			prefix, err := netip.ParsePrefix(entry)
			if err == nil && prefix.Contains(address) {
				return true
			}
		} else if entry == address.String() {
			return true
		}
	}
	return false
}
