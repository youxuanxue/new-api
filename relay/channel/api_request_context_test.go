package channel

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCallerOwnedTransportContextIsOptIn(t *testing.T) {
	service.InitHttpClient()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	for _, enabled := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if enabled {
			ctx = relaycommon.WithUpstreamRequestContext(ctx)
		}
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{}")).WithContext(ctx)
		req, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("{}"))
		require.NoError(t, err)
		resp, err := DoRequest(c, req, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}})
		if enabled {
			require.Error(t, err)
			require.Nil(t, resp)
		} else {
			require.NoError(t, err)
			_ = resp.Body.Close()
		}
	}
}

func TestCallerOwnedTransportCancellationClosesResponseBody(t *testing.T) {
	service.InitHttpClient()
	canceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(canceled)
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{}")).WithContext(relaycommon.WithUpstreamRequestContext(ctx))
	req, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("{}"))
	require.NoError(t, err)
	resp, err := DoRequest(c, req, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}})
	require.NoError(t, err)
	cancel()
	_, err = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.ErrorIs(t, err, context.Canceled)
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("upstream connection was not canceled")
	}
}
