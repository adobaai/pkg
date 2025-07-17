package httpz

import (
	"context"
	"net/http"
	"testing"

	"github.com/adobaai/pkg/testingz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Hello struct {
	Name string
}

type HelloResp struct {
	JSON *Hello
}

func TestJSON(t *testing.T) {
	c := &http.Client{}
	ctx := context.Background()
	name := "Фёдор Миха́йлович Достое́вский"

	t.Run("JSON", func(t *testing.T) {
		testingz.R(JSON[HelloResp](ctx, c, http.MethodPost,
			"https://httpbin.org/anything", Hello{Name: name})).
			NoError(t).
			Do(func(t *testing.T, it *HelloResp) {
				assert.Equal(t, name, it.JSON.Name)
			})
	})

	t.Run("RawJSON", func(t *testing.T) {
		res, err := RawJSON(ctx, c, http.MethodGet, "https://httpbin.org/status/404", nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, res.StatusCode)
	})

	t.Run("JSON2", func(t *testing.T) {
		type ErrorResp struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}

		url := "https://api.weatherapi.com/v1/current.json"
		res, er, err := JSON2[HelloResp, ErrorResp](ctx, c, http.MethodGet, url,
			Hello{Name: name})
		require.NoError(t, err)
		assert.Nil(t, res)
		assert.Equal(t, 401, er.StatusCode)
		assert.Equal(t, 1002, er.T.Error.Code)
		assert.Equal(t, "API key is invalid or not provided.", er.T.Error.Message)

	})
}
