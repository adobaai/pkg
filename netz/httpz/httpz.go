package httpz

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
)

// JSON performs the request with the data marshaled to JSON format
// and unmarshals the response body into a new R.
func JSON[R any](ctx context.Context, c *http.Client, method, url string, data any,
) (res *R, err error) {
	hreq, err := NewJSONRequest(ctx, method, url, data)
	if err != nil {
		return
	}
	return DoJSON[R](c, hreq)
}

// NewJSONRequest returns a new [http.Request] with the given data marshaled to JSON format.
func NewJSONRequest(ctx context.Context, method, url string, data any,
) (res *http.Request, err error) {
	var r io.Reader
	if data != nil {
		bs, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(bs)
	}
	res, err = http.NewRequestWithContext(ctx, method, url, r)
	if err != nil {
		return
	}
	res.Header.Set("Content-Type", "application/json")
	return
}

// DoJSON performs the request and unmarshals the response body into a new R.
func DoJSON[R any](c *http.Client, req *http.Request) (res *R, err error) {
	hres, err := c.Do(req)
	if err != nil {
		return
	}
	return RespJSON[R](hres)
}

// RespJSON unmarshals the response body into a new R and closes the body afterwards.
func RespJSON[R any](r *http.Response) (res *R, err error) {
	defer r.Body.Close()
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		return
	}
	res = new(R)
	err = json.Unmarshal(bs, res)
	return
}
