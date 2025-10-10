package netz

import (
	"context"
	"net"
	"time"
)

func Ping(ctx context.Context, network, address string) (elapsed time.Duration, err error) {
	start := time.Now()
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, network, address)
	elapsed = time.Since(start)
	if err != nil {
		return
	}
	go func() {
		_ = conn.Close()
	}()
	return
}
