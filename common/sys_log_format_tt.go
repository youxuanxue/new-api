//go:build tt
// +build tt

package common

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

func init() {
	sysLogFormatter = formatJSONLog
}

func formatJSONLog(level, msg string) string {
	_, file, line, ok := runtime.Caller(3)
	caller := "unknown"
	if ok {
		parts := strings.Split(file, "/")
		if len(parts) > 1 {
			caller = fmt.Sprintf("%s/%s:%d", parts[len(parts)-2], parts[len(parts)-1], line)
		} else {
			caller = fmt.Sprintf("%s:%d", file, line)
		}
	}
	return fmt.Sprintf(`{"ts":"%s","level":"%s","msg":"%s","caller":"%s"}`,
		time.Now().UTC().Format(time.RFC3339), level, escapeJSONLogString(msg), caller)
}

func escapeJSONLogString(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, c := range s {
		switch c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(c)
		}
	}
	return b.String()
}
