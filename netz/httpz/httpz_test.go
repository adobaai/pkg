package httpz

import (
	"context"
	"net/http"
	"testing"

	"github.com/adobaai/pkg/testingz"
	"github.com/stretchr/testify/assert"
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
	testingz.R(JSON[HelloResp](ctx, c, http.MethodPost,
		"https://httpbin.org/anything", Hello{Name: name})).
		NoError(t).
		Do(func(t *testing.T, it *HelloResp) {
			assert.Equal(t, name, it.JSON.Name)
		})

	testingz.R(RawJSON(ctx, c, http.MethodGet, "https://httpbin.org/status/404", nil)).
		NoError(t).
		Do(func(t *testing.T, it *http.Response) {
			assert.Equal(t, http.StatusNotFound, it.StatusCode)
		})
}
