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
	"log"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Option func(*Client)

func WithLogger(l logr.Logger) Option {
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
	l       logr.Logger
	tr      trace.Tracer
}

func New(webhook string, opts ...Option) (res *Client) {
	res = &Client{
		Webhook: webhook,
		l:       stdr.New(log.Default()).WithValues("pkg", "lark"),
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

	err = c.send(ctx, Message{})
	if IsNoMsgType(err) {
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
	l := c.l.WithValues("do", "Send")
	bs, err := msg.MarshalLark()
	if err != nil {
		return fmt.Errorf("marshal msg: %w", err)
	}

	l.V(6).Info("new request", "body", string(bs))
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
	defer resp.Body.Close()

	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	} else {
		l.V(6).Info("read body", "body", string(buf))
	}

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

// See https://open.feishu.cn/document/client-docs/bot-v3/add-custom-bot.
type Error struct {
	Code int
	Msg  string
}

func (e Error) Error() string {
	return fmt.Sprintf("code: %d, msg: %s", e.Code, e.Msg)
}

func IsNoMsgType(err error) bool {
	var e Error
	ok := errors.As(err, &e)
	if !ok {
		return false
	}
	return e.Code == 19002
}
