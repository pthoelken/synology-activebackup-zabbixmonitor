package synology

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func StringValue(v any) string {
	if v == nil {
		return ""
	}
	switch value := v.(type) {
	case []byte:
		return string(value)
	case string:
		return value
	case int64:
		return strconv.FormatInt(value, 10)
	case int:
		return strconv.Itoa(value)
	case float64:
		if value == float64(int64(value)) {
			return strconv.FormatInt(int64(value), 10)
		}
		return strconv.FormatFloat(value, 'f', -1, 64)
	case bool:
		if value {
			return "1"
		}
		return "0"
	case time.Time:
		return value.Format(time.RFC3339)
	default:
		return fmt.Sprint(value)
	}
}

func Int64Value(v any) int64 {
	s := strings.TrimSpace(StringValue(v))
	if s == "" {
		return 0
	}
	if strings.Contains(s, ".") {
		f, err := strconv.ParseFloat(s, 64)
		if err == nil {
			return int64(f)
		}
	}
	i, _ := strconv.ParseInt(s, 10, 64)
	return i
}

func ParseTimeValue(v any) *time.Time {
	s := strings.TrimSpace(StringValue(v))
	if s == "" || s == "0" {
		return nil
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		var t time.Time
		switch {
		case i > 100000000000000000:
			t = time.Unix(0, i)
		case i > 100000000000:
			t = time.UnixMilli(i)
		case i > 100000000:
			t = time.Unix(i, 0)
		default:
			return nil
		}
		return &t
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006/01/02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return &t
		}
	}
	return nil
}

func RuntimeSeconds(start *time.Time, end *time.Time) int64 {
	if start == nil || end == nil {
		return 0
	}
	if end.Before(*start) {
		return 0
	}
	return int64(end.Sub(*start).Seconds())
}

func AgeSeconds(now time.Time, when *time.Time) int64 {
	if when == nil {
		return 0
	}
	if now.Before(*when) {
		return 0
	}
	return int64(now.Sub(*when).Seconds())
}
