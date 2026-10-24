package storage

import (
	"database/sql"
	"fmt"
	"time"
)

func Date(t time.Time) string { return t.Format("2006-01-02") }
func encodeArgs(args []any) []any {
	out := make([]any, len(args))
	for i, v := range args {
		switch x := v.(type) {
		case time.Time:
			out[i] = x.UTC().UnixMicro()
		case *time.Time:
			if x != nil {
				out[i] = x.UTC().UnixMicro()
			}
		case sql.NullTime:
			if x.Valid {
				out[i] = x.Time.UTC().UnixMicro()
			}
		default:
			out[i] = v
		}
	}
	return out
}

type Row struct {
	row  *sql.Row
	zone *time.Location
}
type Rows struct {
	*sql.Rows
	zone *time.Location
}

func (r *Row) Scan(dest ...any) error  { return scan(r.row.Scan, r.zone, dest...) }
func (r *Rows) Scan(dest ...any) error { return scan(r.Rows.Scan, r.zone, dest...) }
func scan(fn func(...any) error, zone *time.Location, dest ...any) error {
	converted := append([]any(nil), dest...)
	holders := map[int]*any{}
	for i, d := range dest {
		switch d.(type) {
		case *time.Time, *sql.NullTime:
			var raw any
			holders[i] = &raw
			converted[i] = &raw
		}
	}
	if err := fn(converted...); err != nil {
		return err
	}
	for i, h := range holders {
		var value time.Time
		valid := *h != nil
		if valid {
			switch v := (*h).(type) {
			case int64:
				value = time.UnixMicro(v).In(zone)
			case time.Time:
				value = v
			case []byte:
				parsed, e := parseTime(string(v), zone)
				if e != nil {
					return e
				}
				value = parsed
			case string:
				parsed, e := parseTime(v, zone)
				if e != nil {
					return e
				}
				value = parsed
			default:
				return fmt.Errorf("unsupported stored time %T", v)
			}
		}
		switch d := dest[i].(type) {
		case *time.Time:
			if !valid {
				return fmt.Errorf("unexpected null timestamp")
			}
			*d = value
		case *sql.NullTime:
			*d = sql.NullTime{Time: value, Valid: valid}
		}
	}
	return nil
}
func parseTime(v string, zone *time.Location) (time.Time, error) {
	if len(v) == 10 {
		return time.ParseInLocation("2006-01-02", v, zone)
	}
	return time.Parse(time.RFC3339Nano, v)
}
