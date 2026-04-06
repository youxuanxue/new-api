package common

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	AlertInfo     = "info"
	AlertWarning  = "warning"
	AlertCritical = "critical"
)

var (
	feishuWebhookURL    string
	feishuWebhookSecret string
	feishuEnabled       bool

	alertDedup   = make(map[string]time.Time)
	alertDedupMu sync.Mutex
	alertDedupTTL = 5 * time.Minute
)

func InitFeishuAlert() {
	feishuWebhookURL = os.Getenv("FEISHU_WEBHOOK_URL")
	feishuWebhookSecret = os.Getenv("FEISHU_WEBHOOK_SECRET")

	if feishuWebhookURL == "" {
		feishuEnabled = false
		SysLog("FEISHU_WEBHOOK_URL not set, Feishu alerts disabled")
		return
	}
	feishuEnabled = true
	SysLog("Feishu alerts enabled")
}

func FeishuEnabled() bool {
	return feishuEnabled
}

// SendFeishuAlert sends a card message to the configured Feishu webhook.
// Duplicate alerts with the same title are suppressed for 5 minutes.
func SendFeishuAlert(title, content, level string) {
	if !feishuEnabled {
		return
	}

	alertDedupMu.Lock()
	if last, ok := alertDedup[title]; ok && time.Since(last) < alertDedupTTL {
		alertDedupMu.Unlock()
		return
	}
	alertDedup[title] = time.Now()
	// Evict stale entries to prevent unbounded growth
	for k, v := range alertDedup {
		if time.Since(v) > alertDedupTTL {
			delete(alertDedup, k)
		}
	}
	alertDedupMu.Unlock()

	color := levelToColor(level)
	tag := levelToTag(level)

	payload := fmt.Sprintf(`{
  "msg_type": "interactive",
  "card": {
    "header": {
      "title": {"tag": "plain_text", "content": "%s %s"},
      "template": "%s"
    },
    "elements": [
      {
        "tag": "div",
        "text": {"tag": "plain_text", "content": "%s"}
      },
      {
        "tag": "note",
        "elements": [
          {"tag": "plain_text", "content": "TokenKey | %s"}
        ]
      }
    ]
  }
}`, tag, escapeJSON(title), color, escapeJSON(content), time.Now().Format("2006-01-02 15:04:05"))

	go doFeishuPost(payload)
}

func doFeishuPost(payload string) {
	url := feishuWebhookURL

	if feishuWebhookSecret != "" {
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		sign := genFeishuSign(ts, feishuWebhookSecret)
		// Inject timestamp and sign into payload
		payload = injectSignFields(payload, ts, sign)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBufferString(payload))
	if err != nil {
		SysError("feishu alert send failed: " + err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		SysError(fmt.Sprintf("feishu alert non-200: status=%d body=%s", resp.StatusCode, string(body)))
	}
}

func genFeishuSign(timestamp, secret string) string {
	stringToSign := timestamp + "\n" + secret
	h := hmac.New(sha256.New, []byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func injectSignFields(payload, timestamp, sign string) string {
	// Insert timestamp and sign at the top level of the JSON object
	return fmt.Sprintf(`{"timestamp":"%s","sign":"%s",%s`, timestamp, sign, payload[1:])
}

func levelToColor(level string) string {
	switch level {
	case AlertCritical:
		return "red"
	case AlertWarning:
		return "orange"
	default:
		return "blue"
	}
}

func levelToTag(level string) string {
	switch level {
	case AlertCritical:
		return "[CRITICAL]"
	case AlertWarning:
		return "[WARNING]"
	default:
		return "[INFO]"
	}
}

func escapeJSON(s string) string {
	var buf bytes.Buffer
	for _, c := range s {
		switch c {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			buf.WriteRune(c)
		}
	}
	return buf.String()
}
