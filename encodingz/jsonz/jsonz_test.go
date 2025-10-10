package jsonz

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshal(t *testing.T) {
	type T struct {
		A string
		B int
	}
	bs := []byte(`{"a":"hello","b":1}`)
	tt, err := Unmarshal[T](bs)
	require.NoError(t, err)
	assert.Equal(t, T{A: "hello", B: 1}, *tt)

	bs2 := []byte(`{"a":"hello","b":}`)
	tt, err = Unmarshal[T](bs2)
	require.Error(t, err)
	var jerr *json.SyntaxError
	require.ErrorAs(t, err, &jerr)
	assert.Equal(t, "invalid character '}' looking for beginning of value", jerr.Error())
	assert.Nil(t, tt)
}
