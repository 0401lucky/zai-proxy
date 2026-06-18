package model

import "strings"

// 基础模型映射（key 全部小写，不包含标签后缀）
var BaseModelMapping = map[string]string{
	"glm-5.2":                  "glm-5.2",
	"glm-5.1":                  "GLM-5.1",
	"glm-5":                    "glm-5.2",
	"glm-5-turbo":              "GLM-5-Turbo",
	"glm-5v-turbo":             "GLM-5v-Turbo",
	"glm-4.5":                  "0727-360B-API",
	"glm-4.6":                  "GLM-4-6-API-V1",
	"glm-4.7":                  "glm-4.7",
	"glm-4.5-v":                "glm-4.5v",
	"glm-4.6-v":                "glm-4.6v",
	"glm-4.1v-thinking-flashx": "GLM-4.1V-Thinking-FlashX",
	"glm-4.5-air":              "0727-106B-API",
	"glm-4-flash":              "glm-4-flash",
	"glm-4-air":                "glm-4-air-250414",
	"0808-360b-dr":             "0808-360B-DR",
	"deep-research":            "deep-research",
	"zero":                     "zero",
}

// Claude 模型名到 GLM 基础模型名的映射
var ClaudeModelMapping = map[string]string{
	"claude-opus-4-6":            "glm-5.2",
	"claude-opus-4-5-20250514":   "glm-5.2",
	"claude-sonnet-4-6":          "glm-5.2",
	"claude-sonnet-4-5-20241022": "glm-5.2",
	"claude-haiku-4-5":           "glm-5-turbo",
	"claude-haiku-4-5-20251001":  "glm-5-turbo",
	"claude-3-5-sonnet-20241022": "glm-5.2",
	"claude-3-5-haiku-20241022":  "glm-5-turbo",
}

// ResolveClaudeModel maps a Claude model name to a GLM model name with appropriate tags.
func ResolveClaudeModel(model string, thinkingEnabled bool) (resolvedModel string, enableThinking bool) {
	base, ok := ClaudeModelMapping[strings.ToLower(model)]
	if !ok {
		base = "glm-5.2"
	}

	enableThinking = thinkingEnabled
	if strings.Contains(strings.ToLower(model), "opus") {
		enableThinking = true
	}

	resolvedModel = base
	if enableThinking {
		resolvedModel += "-thinking"
	}
	resolvedModel += "-tools"
	return resolvedModel, enableThinking
}

// v1/models 返回的模型列表（全部小写）
var ModelList = []string{
	"glm-5.2",
	"glm-5.2-thinking",
	"glm-5.2-search",
	"glm-5.2-thinking-search",
	"glm-5.2-tools",
	"glm-5.2-tools-thinking",
	"glm-5.1",
	"glm-5.1-thinking",
	"glm-5-turbo",
	"glm-5-turbo-thinking",
	"glm-5v-turbo",
	"glm-4.5",
	"glm-4.6",
	"glm-4.7",
	"glm-4.7-thinking",
	"glm-4.7-thinking-search",
	"glm-4.7-tools",
	"glm-4.7-tools-thinking",
	"glm-5",
	"glm-5-thinking",
	"glm-5-thinking-search",
	"glm-5-tools",
	"glm-5-tools-thinking",
	"glm-4-flash",
	"glm-4-air",
	"glm-4.5-v",
	"glm-4.6-v",
	"glm-4.6-v-thinking",
	"glm-4.1v-thinking-flashx",
	"glm-4.5-air",
	"0808-360b-dr",
	"deep-research",
	"zero",
}

// 解析模型名称，提取基础模型名和标签
// 输入大小写不敏感，输出的 baseModel 为小写
func ParseModelName(model string) (baseModel string, enableThinking bool, enableSearch bool, enableTools bool) {
	enableThinking = false
	enableSearch = false
	enableTools = false
	baseModel = strings.ToLower(model)

	for {
		if strings.HasSuffix(baseModel, "-thinking") {
			enableThinking = true
			baseModel = strings.TrimSuffix(baseModel, "-thinking")
		} else if strings.HasSuffix(baseModel, "-search") {
			enableSearch = true
			baseModel = strings.TrimSuffix(baseModel, "-search")
		} else if strings.HasSuffix(baseModel, "-tools") {
			enableTools = true
			baseModel = strings.TrimSuffix(baseModel, "-tools")
		} else {
			break
		}
	}

	return baseModel, enableThinking, enableSearch, enableTools
}

func IsThinkingModel(model string) bool {
	_, enableThinking, _, _ := ParseModelName(model)
	return enableThinking
}

func IsSearchModel(model string) bool {
	_, _, enableSearch, _ := ParseModelName(model)
	return enableSearch
}

func IsToolsModel(model string) bool {
	_, _, _, enableTools := ParseModelName(model)
	return enableTools
}

func GetTargetModel(model string) string {
	baseModel, _, _, _ := ParseModelName(model)
	if target, ok := BaseModelMapping[baseModel]; ok {
		return target
	}
	return "glm-5.2"
}
