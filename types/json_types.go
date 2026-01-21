package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSONMap represents a JSON object (map) in the database.
// Used for flexible key-value storage like audit log changes.
type JSONMap map[string]interface{}

// Scan implements the sql.Scanner interface for reading from database.
func (j *JSONMap) Scan(val interface{}) error {
	switch v := val.(type) {
	case []byte:
		return json.Unmarshal(v, j)
	case string:
		return json.Unmarshal([]byte(v), j)
	case nil:
		*j = nil
		return nil
	default:
		return fmt.Errorf("cannot scan %T into JSONMap", v)
	}
}

// Value implements the driver.Valuer interface for writing to database.
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// StringArray represents a JSON array of strings in the database.
type StringArray []string

// Scan implements the sql.Scanner interface for reading from database.
func (s *StringArray) Scan(val interface{}) error {
	switch v := val.(type) {
	case []byte:
		return json.Unmarshal(v, s)
	case string:
		return json.Unmarshal([]byte(v), s)
	case nil:
		*s = nil
		return nil
	default:
		return fmt.Errorf("cannot scan %T into StringArray", v)
	}
}

// Value implements the driver.Valuer interface for writing to database.
func (s StringArray) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}
