package lark

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSend(t *testing.T) {
	url := "https://open.feishu.cn/open-apis/bot/v2/hook/your-id"
	c := New(
		url,
		WithTimeout(6*time.Second),
		WithLogger(testr.NewWithOptions(t, testr.Options{
			LogTimestamp: true,
			Verbosity:    9,
		})))

	ctx := context.Background()
	t.Run("card-md", func(t *testing.T) {
		card := new(Card).
			SetHeader("Unit test card", Carmine).
			AddElem(&Markdown{
				Content: "hello **The Witcher 3**",
			})
		require.NoError(t, c.Send(ctx, Message{
			Type: Interactive,
			Card: card,
		}))
	})

	t.Run("card-div", func(t *testing.T) {
		card := NewCard("Unit test div", Indigo).
			AddElem(NewDiv().
				AddField(NewField(true).SetText(NewMDTextf("**Time**:\n%s", time.Now().String()))).
				AddField(NewField(true).SetText(NewText(PlainText, "兄弟们, 下班了."))),
			)
		require.NoError(t, c.Send(ctx, Message{
			Type: Interactive,
			Card: card,
		}))
	})

	t.Run("ping", func(t *testing.T) {
		err := c.Ping(ctx)
		assert.NoError(t, err)
	})
}
