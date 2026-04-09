package common

import "time"

// GracefulShutdownDrain is the HTTP server shutdown timeout after SIGINT/SIGTERM
// while active connections (including SSE) are drained.
const GracefulShutdownDrain = 15 * time.Second
