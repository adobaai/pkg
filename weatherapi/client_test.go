package weatherapi

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCurrent(t *testing.T) {
	key := os.Getenv("WEATHERAPI_KEY")
	c := NewClient(key)
	res, err := c.GetCurrent(context.Background(), WithCity("Chengdu"))
	require.NoError(t, err)
	assert.Equal(t, res.Location.Name, "Chengdu")
	assert.Equal(t, res.Location.Region, "Sichuan")
	assert.NotZero(t, res.Current.Condition.Text)
	assert.NotZero(t, res.Current.TempC)
}
