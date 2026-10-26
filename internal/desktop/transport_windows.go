//go:build windows

package desktop

import (
	"context"
	"github.com/Microsoft/go-winio"
	"io"
)

func dial(ctx context.Context, path string) (io.ReadWriteCloser, error) {
	return winio.DialPipeContext(ctx, path)
}
