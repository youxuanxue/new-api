package common

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// LogWriterMu protects concurrent access to gin.DefaultWriter/gin.DefaultErrorWriter
// during log file rotation. Acquire RLock when reading/writing through the writers,
// acquire Lock when swapping writers and closing old files.
var LogWriterMu sync.RWMutex

// sysLogFormatter controls the output format.  Defaults to plain text; TT
// builds override this to JSON via init() in sys_log_format_tt.go.
var sysLogFormatter func(level, msg string) string

func formatLogLine(level, msg string) string {
	if sysLogFormatter != nil {
		return sysLogFormatter(level, msg)
	}
	return fmt.Sprintf("[SYS] %v | %s", time.Now().Format("2006/01/02 - 15:04:05"), msg)
}

func SysLog(s string) {
	LogWriterMu.RLock()
	_, _ = fmt.Fprintln(gin.DefaultWriter, formatLogLine("info", s))
	LogWriterMu.RUnlock()
}

func SysError(s string) {
	LogWriterMu.RLock()
	_, _ = fmt.Fprintln(gin.DefaultErrorWriter, formatLogLine("error", s))
	LogWriterMu.RUnlock()
}

func FatalLog(v ...any) {
	LogWriterMu.RLock()
	_, _ = fmt.Fprintln(gin.DefaultErrorWriter, formatLogLine("fatal", fmt.Sprint(v...)))
	LogWriterMu.RUnlock()
	os.Exit(1)
}

func LogStartupSuccess(startTime time.Time, port string) {
	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	// Get network IPs
	networkIps := GetNetworkIps()

	LogWriterMu.RLock()
	defer LogWriterMu.RUnlock()

	fmt.Fprintf(gin.DefaultWriter, "\n")
	fmt.Fprintf(gin.DefaultWriter, "  \033[32m%s %s\033[0m  ready in %d ms\n", SystemName, Version, durationMs)
	fmt.Fprintf(gin.DefaultWriter, "\n")

	if !IsRunningInContainer() {
		fmt.Fprintf(gin.DefaultWriter, "  ➜  \033[1mLocal:\033[0m   http://localhost:%s/\n", port)
	}

	for _, ip := range networkIps {
		fmt.Fprintf(gin.DefaultWriter, "  ➜  \033[1mNetwork:\033[0m http://%s:%s/\n", ip, port)
	}

	fmt.Fprintf(gin.DefaultWriter, "\n")
}
