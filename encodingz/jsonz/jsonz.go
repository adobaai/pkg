package jsonz

import "encoding/json"

func Unmarshal[T any](bs []byte) (*T, error) {
	var t T
	if err := json.Unmarshal(bs, &t); err != nil {
		return nil, err
	}
	return &t, nil
}
