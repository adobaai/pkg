package netz

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPing(t *testing.T) {
	ctx := context.Background()

	pingTCP := func(t *testing.T, address string) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		elapsed, err := Ping(ctx, "tcp", address)
		require.NoError(t, err)
		t.Log(elapsed)
	}

	t.Run("DNSPod", func(t *testing.T) {
		pingTCP(t, "119.29.29.29:53")
	})

	t.Run("AliDNS", func(t *testing.T) {
		pingTCP(t, "223.5.5.5:53")
	})

	t.Run("GoogleDNS", func(t *testing.T) {
		pingTCP(t, "1.1.1.1:53")
	})
}
