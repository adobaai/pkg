package lark

import (
	"encoding/json"
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

type MessageType string

const (
	Interactive MessageType = "interactive"
)

// See https://open.feishu.cn/document/ukTMukTMukTM/ucTM5YjL3ETO24yNxkjN
type Message struct {
	Type MessageType
	*Card
}

func (m *Message) MarshalLark() ([]byte, error) {
	mp := map[string]any{
		"msg_type": m.Type,
	}
	if m.Card != nil {
		mp["card"] = m.Card.Render()
	}
	return json.Marshal(mp)
}

// Card is the card message.
// See doc: https://open.feishu.cn/document/ukTMukTMukTM/uczM3QjL3MzN04yNzcDN.
//
// Thanks to https://github.com/go-lark/lark for the card builder idea.
// See https://github.com/go-lark/examples/blob/main/interactive-message/main.go for an
// example.
type Card struct {
	header CardHeader
	elems  []Element
}

func NewCard(title string, template CardTemplate) *Card {
	return new(Card).SetHeader(title, template)
}

func (c *Card) SetHeader(title string, template CardTemplate) *Card {
	c.header = CardHeader{
		Template: template,
		Title: CardHeaderTitle{
			Tag:     "plain_text",
			Content: title,
		},
	}
	return c
}

func (c *Card) AddElem(e Element) *Card {
	c.elems = append(c.elems, e)
	return c
}

func (c *Card) Render() any {
	var elems []any
	for _, e := range c.elems {
		elems = append(elems, e.Render())
	}

	return map[string]any{
		"Header":   c.header,
		"Elements": elems,
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

type CardHeaderTitle struct {
	Tag     string
	Content string
}

type CardHeader struct {
	Title    CardHeaderTitle
	Template CardTemplate
}

type tagger struct {
	Tag string
}

// Markdown represents a Markdown element.
// See https://open.larksuite.com/document/ukTMukTMukTM/uADOwUjLwgDM14CM4ATN
type Markdown struct {
	tagger
	Content string
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

func (d *Div) SetText(t *Text) *Div {
	d.Text = t
	return d
}

func (d *Div) AddField(f *Field) *Div {
	d.Fields = append(d.Fields, f)
	return d
}

func (d *Div) Render() any {
	return map[string]any{
		"tag":    "div",
		"text":   d.Text,
		"fields": collections.Map(d.Fields, func(it *Field) any { return it.Render() }),
	}
}

// Field is the card field.
// See https://open.larksuite.com/document/common-capabilities/message-card/message-cards-content/embedded-non-interactive-elements/field
type Field struct {
	IsShort bool // 是否并排布局
	Text    *Text
}

func NewField(isShort bool) *Field {
	return &Field{
		IsShort: isShort,
	}
}

func (f *Field) SetText(t *Text) *Field {
	f.Text = t
	return f
}

func (f *Field) Render() any {
	return map[string]any{
		"is_short": f.IsShort,
		"text":     f.Text.Render(),
	}
}

type TextType int8

const (
	PlainText TextType = iota
	MDText             // Markdown
)

func (tt TextType) Tag() string {
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
	tagger
	Type    TextType
	Content string
	Lines   int // content 显示的行数
}

func NewText(t TextType, content string) *Text {
	return &Text{
		Type:    t,
		Content: content,
	}
}

// NewMDText new a markdown [Text].
func NewMDText(s string) *Text {
	return &Text{
		Type:    MDText,
		Content: s,
	}
}

func NewMDTextf(format string, a ...any) *Text {
	return &Text{
		Type:    MDText,
		Content: fmt.Sprintf(format, a...),
	}
}

func (t *Text) SetLines(lines int) *Text {
	t.Lines = lines
	return t
}

func (t *Text) Render() any {
	t.Tag = t.Type.Tag()
	return t
}
