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

// JSON2 likes the [JSON], but can specify the error response type.
func JSON2[R, E any](ctx context.Context, c *http.Client, method, url string, data any,
) (res *R, er ErrorResp[E], err error) {
	hreq, err := NewJSONRequest(ctx, method, url, data)
	if err != nil {
		return
	}
	return DoJSON2[R, E](c, hreq)
}

// RawJSON performs the request with the data marshaled to JSON format
// and returns the raw response.
func RawJSON(ctx context.Context, c *http.Client, method, url string, data any,
) (res *http.Response, err error) {
	hreq, err := NewJSONRequest(ctx, method, url, data)
	if err != nil {
		return
	}

	return c.Do(hreq)
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

// ErrorResp is the error response.
type ErrorResp[T any] struct {
	*http.Response
	T *T
}

// IsZero reports whether the ErrorResp is zero.
func (er ErrorResp[T]) IsZero() bool {
	return er.Response == nil && er.T == nil
}

// DoJSON2 likes the [DoJSON], but can specify the error response type.
func DoJSON2[R, E any](c *http.Client, req *http.Request) (res *R, er ErrorResp[E], err error) {
	hres, err := c.Do(req)
	if err != nil {
		return
	}
	if hres.StatusCode >= http.StatusBadRequest {
		e, err := RespJSON[E](hres)
		return nil, ErrorResp[E]{hres, e}, err
	}
	res, err = RespJSON[R](hres)
	return
}

// RespJSON unmarshals the response body into a new R and closes the body afterwards.
func RespJSON[R any](r *http.Response) (res *R, err error) {
	defer r.Body.Close()
	res = new(R)
	err = json.NewDecoder(r.Body).Decode(res)
	return
}
