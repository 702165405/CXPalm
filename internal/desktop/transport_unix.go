//go:build !windows

package desktop

import (
	"context"
	"io"
	"net"
)

func dial(ctx context.Context, path string) (io.ReadWriteCloser, error) {
	return (&net.Dialer{}).DialContext(ctx, "unix", path)
}
