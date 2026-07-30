package volcengine

import (
	"testing"

	channelconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
)

func TestAdaptorGetRequestURLAgentPlanResponses(t *testing.T) {
	url, err := (&Adaptor{}).GetRequestURL(&common.RelayInfo{
		ChannelMeta: &common.ChannelMeta{
			ChannelBaseUrl: "doubao-agent-plan",
		},
		RelayMode: constant.RelayModeResponses,
	})
	if err != nil {
		t.Fatalf("GetRequestURL returned error: %v", err)
	}
	want := channelconstant.ChannelSpecialBases["doubao-agent-plan"].OpenAIBaseURL + "/responses"
	if url != want {
		t.Fatalf("GetRequestURL = %q, want %q", url, want)
	}
}
