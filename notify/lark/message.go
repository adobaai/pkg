package lark

import (
	"fmt"

	"github.com/adobaai/pkg/collections"
)

var (
	_ Element = (*Markdown)(nil)
	_ Element = (*Div)(nil)
)

type Element interface {
	Render() any
}

// Message represents a Lark message.
//
// See https://open.feishu.cn/document/ukTMukTMukTM/ucTM5YjL3ETO24yNxkjN
type Message interface {
	RenderMessage() any
}

type MessageType string

const (
	Interactive MessageType = "interactive"
)

type Direction string

const (
	Vertical   Direction = "vertical"
	Horizontal Direction = "horizontal"
)

type TextMessage struct {
	Text string `json:"text"`
}

func (t *TextMessage) RenderMessage() any {
	return map[string]any{
		"msg_type": "text",
		"content": map[string]string{
			"text": t.Text,
		},
	}
}

type CardTemplate string

const (
	Red       CardTemplate = "red"       // 红色
	Blue      CardTemplate = "blue"      // 蓝色
	Indigo    CardTemplate = "indigo"    // 靛蓝色
	Carmine   CardTemplate = "carmine"   // 胭脂红
	Wathet    CardTemplate = "wathet"    // 浅蓝色
	Violet    CardTemplate = "violet"    // 堇紫色
	Turquoise CardTemplate = "turquoise" // 绿松色
)

// CardMessage is the card message.
// See doc: https://open.feishu.cn/document/ukTMukTMukTM/uczM3QjL3MzN04yNzcDN.
//
// Thanks to https://github.com/go-lark/lark for the card builder idea.
// See https://github.com/go-lark/examples/blob/main/interactive-message/main.go for an
// example.
type CardMessage struct {
	Schema string
	Config *CardConfig
	Header *CardHeader
	Body   *CardBody
}

type CardConfig struct {
	Style any
}

type CardHeader struct {
	Title    *Text        `json:"title"`
	Subtitle *Text        `json:"subtitle"`
	Template CardTemplate `json:"template"`
	Padding  string       `json:"padding"`
}

type CardBody struct {
	Direction Direction `json:"direction"`
	Padding   string    `json:"padding"`
	Elements  []Element `json:"elements"`
}

func (m *CardMessage) RenderMessage() any {
	return map[string]any{
		"msg_type": "interactive",
		"card":     m.Render(),
	}
}

func (m *CardMessage) Render() any {
	return map[string]any{
		"schema": m.Schema,
		"config": m.Config,
		"header": m.Header,
		"body":   m.Body.Render(),
	}
}

func (b *CardBody) Render() any {
	return map[string]any{
		"direction": b.Direction,
		"padding":   b.Padding,
		"elements":  collections.Map(b.Elements, func(it Element) any { return it.Render() }),
	}
}

type NewCardOption func(*CardMessage)

// NewCard creates a new card with the given title and template.
func NewCardMessage(opts ...NewCardOption) *CardMessage {
	m := &CardMessage{
		Schema: "2.0",
		Body:   NewCardBody(),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func NewCardBody() *CardBody {
	return &CardBody{
		Direction: Vertical,
	}
}

// NewMDCard creates a new interactive card message with markdown content.
func NewMDCard(title, subtitle, content string, template CardTemplate) *CardMessage {
	return NewCardMessage().
		SetMDTitle(title, subtitle, template).
		AddMarkdown(content)
}

func (c *CardMessage) SetHeader(h *CardHeader) *CardMessage {
	c.Header = h
	return c
}

func (c *CardMessage) SetMDTitle(title, subtitle string, template CardTemplate) *CardMessage {
	if c.Header == nil {
		c.Header = &CardHeader{}
	}
	c.Header.Title = NewMDText(title)
	c.Header.Subtitle = NewMDText(subtitle)
	c.Header.Template = template
	return c
}

func (c *CardMessage) AddMarkdown(format string, a ...any) *CardMessage {
	s := fmt.Sprintf(format, a...)
	return c.AddElems(NewMarkdown(s))
}

func (c *CardMessage) AddElems(es ...Element) *CardMessage {
	if c.Body == nil {
		c.Body = NewCardBody()
	}
	c.Body.AddElems(es...)
	return c
}

func (c *CardBody) AddElems(es ...Element) *CardBody {
	c.Elements = append(c.Elements, es...)
	return c
}

type tagger struct {
	Tag string `json:"tag"`
}

// Markdown represents a Markdown element.
// See https://open.larksuite.com/document/ukTMukTMukTM/uADOwUjLwgDM14CM4ATN
type Markdown struct {
	tagger
	Content string `json:"content"`
}

func NewMarkdown(content string) *Markdown {
	return &Markdown{
		Content: content,
	}
}

func (m *Markdown) Render() any {
	m.Tag = "markdown"
	return m
}

// Div
// https://open.larksuite.com/document/common-capabilities/message-card/message-cards-content/content-module
type Div struct {
	Text   *Text
	Fields []*Field
}

func NewDiv() *Div {
	return new(Div)
}

// SetText sets the text content for the div.
func (d *Div) SetText(t *Text) *Div {
	d.Text = t
	return d
}

// SetPlainText sets plain text content for the div.
func (d *Div) SetPlainText(content string) *Div {
	return d.SetText(NewText(PlainText, content))
}

// SetMDText sets markdown text content for the div.
func (d *Div) SetMDText(content string) *Div {
	return d.SetText(NewText(MDText, content))
}

// AddField adds a field to the div.
func (d *Div) AddField(f *Field) *Div {
	d.Fields = append(d.Fields, f)
	return d
}

// AddText adds a text field to the div.
func (d *Div) AddText(isShort bool, textType TextTag, content string) *Div {
	return d.AddField(NewField(isShort).SetText(NewText(textType, content)))
}

// AddPlainText adds a plain text field to the div.
func (d *Div) AddPlainText(isShort bool, content string) *Div {
	return d.AddText(isShort, PlainText, content)
}

// AddMDText adds a markdown text field to the div.
func (d *Div) AddMDText(isShort bool, content string) *Div {
	return d.AddText(isShort, MDText, content)
}

func (d *Div) AddMDTextf(isShort bool, format string, a ...any) *Div {
	s := fmt.Sprintf(format, a...)
	return d.AddText(isShort, MDText, s)
}

func (d *Div) Render() any {
	return map[string]any{
		"tag":    "div",
		"text":   d.Text,
		"fields": d.Fields,
	}
}

// Field is the card field.
// See https://open.larksuite.com/document/common-capabilities/message-card/message-cards-content/embedded-non-interactive-elements/field
type Field struct {
	IsShort bool  `json:"is_short"` // 是否并排布局
	Text    *Text `json:"text"`
}

func NewField(isShort bool) *Field {
	return &Field{
		IsShort: isShort,
	}
}

// SetText sets the text content for the field.
func (f *Field) SetText(t *Text) *Field {
	f.Text = t
	return f
}

type TextTag string

const (
	PlainText TextTag = "plain_text"
	MDText    TextTag = "lark_md"
)

func (tt TextTag) Tag() string {
	switch tt {
	case PlainText:
		return "plain_text"
	case MDText:
		return "lark_md"
	default:
		return "unknown"
	}
}

// Text is the card text.
// See https://open.larksuite.com/document/common-capabilities/message-card/message-cards-content/embedded-non-interactive-elements/text.
type Text struct {
	Tag     TextTag `json:"tag"`
	Content string  `json:"content"`
	Lines   int     `json:"lines"` // content 显示的行数
}

func NewText(t TextTag, content string) *Text {
	return &Text{
		Tag:     t,
		Content: content,
	}
}

func NewPlainText(s string) *Text {
	return NewText(PlainText, s)
}

// NewMDText new a markdown [Text].
func NewMDText(s string) *Text {
	return &Text{
		Tag:     MDText,
		Content: s,
	}
}

func NewMDTextf(format string, a ...any) *Text {
	return &Text{
		Tag:     MDText,
		Content: fmt.Sprintf(format, a...),
	}
}

func (t *Text) SetLines(lines int) *Text {
	t.Lines = lines
	return t
}
