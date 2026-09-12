package util

import "encoding/json"

func UnmarshalJSON[T any](s string, dst *T) error {
	return json.Unmarshal([]byte(s), dst)
}
