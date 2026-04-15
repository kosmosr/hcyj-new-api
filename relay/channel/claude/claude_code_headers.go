package claude

import (
	cryptoRand "crypto/rand"
	"fmt"
	"net/http"
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/model_setting"

	"github.com/gin-gonic/gin"
)

// Claude Code 必要的 anthropic-beta 令牌
var betaTokensHaiku = []string{
	"oauth-2025-04-20",
	"interleaved-thinking-2025-05-14",
}

var betaTokensDefault = []string{
	"claude-code-20250219",
	"oauth-2025-04-20",
	"interleaved-thinking-2025-05-14",
	"prompt-caching-scope-2026-01-05",
}

// InjectClaudeCodeHeaders 为 Anthropic 直连渠道注入 Claude Code CLI 特征请求头
func InjectClaudeCodeHeaders(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) {
	version := model_setting.GetClaudeSettings().GetClaudeCodeVersion()

	// User-Agent
	req.Set("User-Agent", fmt.Sprintf("claude-cli/%s (external, cli)", version))

	// Stainless SDK headers
	req.Set("X-Stainless-OS", "darwin")
	req.Set("X-Stainless-Arch", "arm64")
	req.Set("X-Stainless-Node-Version", "v22.15.0")
	req.Set("X-Stainless-Runtime", "node")
	req.Set("X-Stainless-Package-Version", version)
	req.Set("X-Stainless-Retry-Header", `{"max_retries":2}`)

	// Session & Request IDs
	req.Set("X-Claude-Code-Session-Id", randomUUIDForHeader())
	req.Set("X-Request-Id", randomUUIDForHeader())

	// 合并 anthropic-beta
	existingBeta := c.Request.Header.Get("anthropic-beta")
	mergedBeta := mergeAnthropicBetaTokens(existingBeta, info.UpstreamModelName)
	req.Set("anthropic-beta", mergedBeta)

	// 模型特定 headers
	model_setting.GetClaudeSettings().WriteHeaders(info.OriginModelName, req)
}

// mergeAnthropicBetaTokens 根据模型合并 anthropic-beta 令牌，去重
func mergeAnthropicBetaTokens(existing string, model string) string {
	required := betaTokensDefault
	if strings.Contains(strings.ToLower(model), "haiku") {
		required = betaTokensHaiku
	}

	existingSet := make(map[string]bool)
	var tokens []string
	if existing != "" {
		for _, t := range strings.Split(existing, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				existingSet[t] = true
				tokens = append(tokens, t)
			}
		}
	}

	for _, t := range required {
		if !existingSet[t] {
			tokens = append(tokens, t)
		}
	}
	return strings.Join(tokens, ",")
}

// randomUUIDForHeader 生成随机 UUID v4 用于请求头
func randomUUIDForHeader() string {
	var b [16]byte
	cryptoRandRead(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// cryptoRandRead 包装 crypto/rand.Read，忽略错误（crypto/rand 在主流平台不会失败）
func cryptoRandRead(b []byte) {
	_, _ = cryptoRand.Read(b)
}
