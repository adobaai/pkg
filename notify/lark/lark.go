// Package lark implements sending notifications to lark.
// It references the code from:
// https://github.com/megaease/easeprobe/tree/v2.0.0/notify/lark
package lark

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/multierr"
)

var (
	ErrCreateMessage = Error{Code: 11246, Msg: ""}
)

type Option func(*Client)

func WithLogger(l *slog.Logger) Option {
	return func(c *Client) {
		c.l = l
	}
}
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.timeout = d
	}
}

type Client struct {
	Webhook string

	timeout time.Duration
	l       *slog.Logger
	tr      trace.Tracer
}

func New(webhook string, opts ...Option) (res *Client) {
	res = &Client{
		Webhook: webhook,
		l:       slog.Default().With("pkg", "lark"),
		tr:      otel.Tracer("notify.lark"),
	}
	for _, opt := range opts {
		opt(res)
	}
	return
}

func (c *Client) Ping(ctx context.Context) (err error) {
	ctx, _, end := c.trace(ctx, "Ping", &err)
	defer end()

	err = c.send(ctx, NewCardMessage())
	if errors.Is(err, ErrCreateMessage) {
		err = nil
	}
	return
}

func (c *Client) Send(ctx context.Context, msg Message) (err error) {
	ctx, _, end := c.trace(ctx, "Send", &err)
	defer end()
	return c.send(ctx, msg)
}

func (c *Client) trace(ctx context.Context, name string, perr *error) (rctx context.Context, span trace.Span, end func()) {
	rctx, span = c.tr.Start(ctx, "lark: "+name, trace.WithSpanKind(trace.SpanKindClient))
	end = func() {
		if perr != nil && *perr != nil {
			span.SetStatus(codes.Error, (*perr).Error())
			span.RecordError(*perr)
		}
		span.End()
	}
	return
}

func (c *Client) send(ctx context.Context, msg Message) (err error) {
	l := c.l.With("do", "Send")

	bs, err := json.Marshal(msg.RenderMessage())
	if err != nil {
		return fmt.Errorf("marshal msg: %w", err)
	}

	l.Debug("new request", "body", string(bs))
	req, err := http.NewRequestWithContext(ctx,
		http.MethodPost, c.Webhook, bytes.NewReader(bs))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")
	req.Close = true

	client := &http.Client{Timeout: c.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("do: %w", err)
	}
	defer multierr.AppendFunc(&err, resp.Body.Close)

	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	// l.Debug("read body", "status", resp.Status, "body", string(buf))

	// Refer https://developer.mozilla.org/en-US/docs/Web/API/Response/ok:
	// The ok read-only property of the Response interface contains a Boolean stating
	// whether the response was successful (status in the range 200-299) or not.
	if resp.StatusCode >= 300 {
		return errors.New("bad status: " + resp.Status)
	}

	var e Error
	if err = json.Unmarshal(buf, &e); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	if e.Code == 0 {
		return nil
	}
	return e
}

// See https://open.feishu.cn/document/server-docs/api-call-guide/generic-error-code
type Error struct {
	Code int
	Msg  string
}

func (e Error) Error() string {
	return fmt.Sprintf("code: %d, msg: %s", e.Code, e.Msg)
}

func (e Error) Is(target error) bool {
	if t := target.(Error); t.Code == e.Code {
		return true
	}
	return false
}

func IsNoMsgType(err error) bool {
	var e Error
	ok := errors.As(err, &e)
	if !ok {
		return false
	}
	return e.Code == 19002
}
