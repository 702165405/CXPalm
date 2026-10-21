package desktop

import (
	"errors"
	"fmt"
)

// Rejected means a known precondition prevented this mutation, not a timeout.
type Rejected struct {
	Code, Message string
}

func (e *Rejected) Error() string { return e.Message }

type metadataError struct {
	Code    int
	Message string
}

func (e *metadataError) Error() string { return "official metadata API: " + e.Message }

func archiveFailure(err error, id string) error {
	var api *metadataError
	if errors.As(err, &api) && api.Code == -32600 && api.Message == fmt.Sprintf("thread %s already has an active writer", id) {
		return &Rejected{Code: "active_writer", Message: ArchiveWriterMessage}
	}
	return err
}

const ArchiveWriterMessage = "归档操作未执行：官方桌面仍持有此会话的写入权，空闲不代表写锁已释放。请在官方桌面完成归档；此请求不会自动重发。"
