package utils

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
)

type StringSlice []string

func (s StringSlice) Value() (driver.Value, error) {
	if len(s) == 0 {
		return nil, nil
	}
	return strings.Join(s, ","), nil
}

func (s *StringSlice) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		if str == "" {
			*s = StringSlice{}
			return nil
		}
		// Split by comma and trim spaces from each tag
		tags := strings.Split(str, ",")
		for i, tag := range tags {
			tags[i] = strings.TrimSpace(tag)
		}
		*s = tags
		return nil
	}

	// If string unmarshal fails, try as string slice
	var strSlice []string
	if err := json.Unmarshal(data, &strSlice); err != nil {
		return err
	}
	*s = strSlice
	return nil
}

func (s *StringSlice) Scan(value interface{}) error {
	if value == nil {
		*s = StringSlice{}
		return nil
	}

	switch v := value.(type) {
	case []byte:
		if len(v) == 0 {
			*s = StringSlice{}
			return nil
		}
		*s = StringSlice(strings.Split(string(v), ","))
		// Trim spaces from each tag
		for i, tag := range *s {
			(*s)[i] = strings.TrimSpace(tag)
		}
		return nil
	case string:
		if v == "" {
			*s = StringSlice{}
			return nil
		}
		*s = StringSlice(strings.Split(v, ","))
		// Trim spaces from each tag
		for i, tag := range *s {
			(*s)[i] = strings.TrimSpace(tag)
		}
		return nil
	default:
		return fmt.Errorf("unsupported Scan, storing %T into type *StringSlice", value)
	}
}
