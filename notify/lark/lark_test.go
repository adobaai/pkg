package lark

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSend(t *testing.T) {
	// url := "https://open.feishu.cn/open-apis/bot/v2/hook/your-id"
	url := os.Getenv("LARK_BOT_URL")
	c := New(
		url,
		WithTimeout(6*time.Second),
		WithLogger(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))))

	ctx := context.Background()
	t.Run("CardMDLegacy", func(t *testing.T) {
		card := NewCardMessage().
			SetMDTitle("Unit _test_ card", "", Blue).
			AddElems(NewMarkdown("hello **The Witcher 3**"))
		require.NoError(t, c.Send(ctx, card))
	})

	t.Run("CardMDErgonomic", func(t *testing.T) {
		card := NewCardMessage().
			AddMarkdown("hello **The Witcher 3**")
		require.NoError(t, c.Send(ctx, card))
	})

	t.Run("CardDIVLegacy", func(t *testing.T) {
		card := NewCardMessage().
			AddElems(NewDiv().
				AddField(NewField(true).SetText(NewMDTextf("**Time**:\n%s", time.Now().String()))).
				AddField(NewField(true).SetText(NewText(PlainText, "兄弟们, 下班了."))),
			)
		require.NoError(t, c.Send(ctx, card))
	})

	t.Run("CardDIVErgonomic", func(t *testing.T) {
		card := NewCardMessage()
		div := NewDiv().
			AddMDTextf(true, "**Time**:\n%s", time.Now().String()).
			AddPlainText(true, "兄弟们, 下班了.")
		card.AddElems(div)
		require.NoError(t, c.Send(ctx, card))
	})

	t.Run("Simple", func(t *testing.T) {
		card := NewMDCard("Simple Alert", "", "This is a simple alert message", Blue)
		require.NoError(t, c.Send(ctx, card))
	})

	t.Run("Alert", func(t *testing.T) {
		card := NewCardMessage().
			SetMDTitle("Error Alert", "", Red).
			AddMarkdown("Something went _wrong_!")

		require.NoError(t, c.Send(ctx, card))
	})

	t.Run("Complex", func(t *testing.T) {
		card := NewCardMessage().
			SetMDTitle("Complex Message", "", Turquoise).
			AddMarkdown("## Main Content\nThis is a complex message with multiple elements.")

		div := NewDiv().
			SetMDText("### Details:").
			AddMDText(false, "**Status**: Running").
			AddMDText(false, "**Time**: "+time.Now().Format(time.RFC3339))

		div2 := NewDiv().
			AddPlainText(true, "Field 1: Value 1").
			AddPlainText(true, "Field 2: Value 2")

		card.AddElems(div, div2)
		require.NoError(t, c.Send(ctx, card))
	})

	t.Run("Ping", func(t *testing.T) {
		err := c.Ping(ctx)
		assert.NoError(t, err)
	})
}
