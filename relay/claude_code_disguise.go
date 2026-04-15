package relay

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"
	xxhash "github.com/cespare/xxhash/v2"
)

const (
	cchPlaceholder           = "00000"
	versionSuffixPlaceholder = "000"
)

// generateClaudeCodeUserIdDynamic 基于 userId 确定性生成唯一的 Claude Code user_id
func generateClaudeCodeUserIdDynamic(userId int) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("claude-code-user-%d", userId)))
	hex64 := fmt.Sprintf("%x", hash[:])
	sessionUUID := deterministicUUIDv4(fmt.Sprintf("session_%d", userId))
	return fmt.Sprintf("user_%s_account__session_%s", hex64, sessionUUID)
}

// computeCCH 基于 xxhash64 计算 Content Commitment Hash
// 返回 5 字符 cch 和 3 字符版本后缀
func computeCCH(data []byte) (cch string, versionSuffix string) {
	h := xxhash.Sum64(data)
	cch = fmt.Sprintf("%05x", h&0xFFFFF)
	versionSuffix = fmt.Sprintf("%03x", (h>>20)&0xFFF)
	return
}

// computeAndReplaceCCH 在序列化后的 JSON 中计算并替换 CCH 占位符
func computeAndReplaceCCH(jsonData []byte, version string) []byte {
	cch, vSuffix := computeCCH(jsonData)
	result := bytes.Replace(jsonData,
		[]byte(version+"."+versionSuffixPlaceholder),
		[]byte(version+"."+vSuffix), 1)
	result = bytes.Replace(result,
		[]byte("cch="+cchPlaceholder),
		[]byte("cch="+cch), 1)
	return result
}

// InjectClaudeCodeBody 为 Anthropic Claude 请求注入 metadata 和 system 消息
func InjectClaudeCodeBody(request *dto.ClaudeRequest, userId int) {
	version := model_setting.GetClaudeSettings().GetClaudeCodeVersion()

	// 注入 metadata（动态 user_id）
	if len(request.Metadata) == 0 {
		uid := generateClaudeCodeUserIdDynamic(userId)
		request.Metadata = json.RawMessage(fmt.Sprintf(`{"user_id":"%s"}`, uid))
	}

	// 构建 system 消息（使用占位符，后续由 computeAndReplaceCCH 替换实际值）
	billingText := fmt.Sprintf(
		"x-anthropic-billing-header: cc_version=%s.%s; cc_entrypoint=cli; cch=%s;",
		version, versionSuffixPlaceholder, cchPlaceholder)

	claudeCodeSystem := []dto.ClaudeMediaMessage{
		{
			Type: dto.ContentTypeText,
			Text: common.GetPointer(billingText),
		},
		{
			Type:         dto.ContentTypeText,
			Text:         common.GetPointer("You are Claude Code, Anthropic's official CLI for Claude."),
			CacheControl: json.RawMessage(`{"type":"ephemeral"}`),
		},
	}

	// 合并 system 消息（Claude Code 消息在最前面）
	if request.System == nil {
		request.System = claudeCodeSystem
	} else if request.IsStringSystem() {
		existing := strings.TrimSpace(request.GetStringSystem())
		if existing == "" {
			request.System = claudeCodeSystem
		} else {
			existingMsg := dto.ClaudeMediaMessage{Type: dto.ContentTypeText}
			existingMsg.SetText(existing)
			request.System = append(claudeCodeSystem, existingMsg)
		}
	} else {
		existingSystem := request.ParseSystem()
		if len(existingSystem) == 0 {
			request.System = claudeCodeSystem
		} else {
			request.System = append(claudeCodeSystem, existingSystem...)
		}
	}
}
