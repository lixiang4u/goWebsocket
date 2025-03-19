package goWebsocket

import (
	"encoding/json"
	"github.com/google/uuid"
	"os"
	"path/filepath"
	"strings"
)

func UUID(length ...int) string {
	var s = strings.ReplaceAll(uuid.NewString(), "-", "")
	if len(length) > 0 {
		return s[:length[0]]
	}
	return s
}

func ToJson(v interface{}) string {
	//buf, _ := json.MarshalIndent(v, "", "\t")
	buf, _ := json.Marshal(v)
	//log.Println("[JSON]", string(buf))
	return string(buf)
}

func ToBuff(v interface{}) []byte {
	buff, _ := json.Marshal(v)
	return buff
}

func AppPath() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return ""
	}
	return dir
}
