package go_websocket

import (
	"fmt"
	"github.com/google/uuid"
	"log"
	"strings"
)

func UUID(length ...int) string {
	var s = strings.ReplaceAll(uuid.NewString(), "-", "")
	if len(length) > 0 {
		return s[:length[0]]
	}
	return s
}

func Log2(format string, v ...interface{}) {
	log.Println(fmt.Sprintf(format, v...))
}