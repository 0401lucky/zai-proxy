package upstream

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/corpix/uarand"
	"github.com/google/uuid"

	"zai-proxy/internal/auth"
	"zai-proxy/internal/config"
	"zai-proxy/internal/logger"
	"zai-proxy/internal/model"
	"zai-proxy/internal/proxy"
	builtintools "zai-proxy/internal/tools"
	"zai-proxy/internal/version"
)

func ExtractLatestUserContent(messages []model.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			text, _ := messages[i].ParseContent()
			return text
		}
	}
	return ""
}

func ExtractAllImageURLs(messages []model.Message) []string {
	var allImageURLs []string
	for _, msg := range messages {
		_, imageURLs := msg.ParseContent()
		allImageURLs = append(allImageURLs, imageURLs...)
	}
	return allImageURLs
}

func buildChatQuery(token, userID, requestID, chatID, userAgent string, timestamp int64) string {
	currentURL := fmt.Sprintf("https://chat.z.ai/c/%s", chatID)
	values := url.Values{}
	values.Set("timestamp", fmt.Sprintf("%d", timestamp))
	values.Set("requestId", requestID)
	values.Set("user_id", userID)
	values.Set("version", "0.0.1")
	values.Set("platform", "web")
	values.Set("token", token)
	values.Set("user_agent", userAgent)
	values.Set("language", "zh-CN")
	values.Set("languages", "zh-CN,zh,en")
	values.Set("timezone", "Asia/Shanghai")
	values.Set("cookie_enabled", "true")
	values.Set("screen_width", "1920")
	values.Set("screen_height", "1080")
	values.Set("screen_resolution", "1920x1080")
	values.Set("viewport_height", "900")
	values.Set("viewport_width", "1440")
	values.Set("viewport_size", "1440x900")
	values.Set("color_depth", "24")
	values.Set("pixel_ratio", "1")
	values.Set("current_url", currentURL)
	values.Set("pathname", fmt.Sprintf("/c/%s", chatID))
	values.Set("search", "")
	values.Set("hash", "")
	values.Set("host", "chat.z.ai")
	values.Set("hostname", "chat.z.ai")
	values.Set("protocol", "https:")
	values.Set("referrer", "")
	values.Set("title", "Z.ai - Advanced AI Chatbot & Agent powered by GLM-5.2")
	values.Set("timezone_offset", "-480")
	values.Set("local_time", time.Now().Format(time.RFC3339Nano))
	values.Set("utc_time", time.Now().UTC().Format(time.RFC3339Nano))
	values.Set("is_mobile", "false")
	values.Set("is_touch", "false")
	values.Set("max_touch_points", "0")
	values.Set("browser_name", "Chrome")
	values.Set("os_name", "Windows")
	values.Set("signature_timestamp", fmt.Sprintf("%d", timestamp))
	return values.Encode()
}

func MakeUpstreamRequest(token string, messages []model.Message, modelName string, tools []model.Tool, toolChoice interface{}) (*http.Response, string, error) {
	payload, err := auth.DecodeJWTPayload(token)
	if err != nil || payload == nil {
		return nil, "", fmt.Errorf("invalid token")
	}

	userID := payload.ID
	chatID := uuid.New().String()
	timestamp := time.Now().UnixMilli()
	requestID := uuid.New().String()
	userMsgID := uuid.New().String()
	assistantMsgID := uuid.New().String()

	targetModel := model.GetTargetModel(modelName)
	latestUserContent := ExtractLatestUserContent(messages)
	imageURLs := ExtractAllImageURLs(messages)

	signature := auth.GenerateSignature(userID, requestID, latestUserContent, timestamp)
	userAgent := uarand.GetRandom()
	requestURL := config.ChatEndpointURL() + "?" + buildChatQuery(token, userID, requestID, chatID, userAgent, timestamp)

	enableThinking := model.IsThinkingModel(modelName)
	autoWebSearch := model.IsSearchModel(modelName)
	if targetModel == "glm-4.5v" || targetModel == "glm-4.6v" || targetModel == "GLM-5v-Turbo" || targetModel == "GLM-4.1V-Thinking-FlashX" {
		autoWebSearch = false
	}

	var mcpServers []string
	if targetModel == "glm-4.6v" || targetModel == "GLM-5v-Turbo" || targetModel == "GLM-4.1V-Thinking-FlashX" {
		mcpServers = []string{"vlm-image-search", "vlm-image-recognition", "vlm-image-processing"}
	}

	urlToFileID := make(map[string]string)
	var filesData []map[string]interface{}
	if len(imageURLs) > 0 {
		files, _ := UploadImages(token, imageURLs)
		for i, f := range files {
			if i < len(imageURLs) {
				urlToFileID[imageURLs[i]] = f.ID
			}
			filesData = append(filesData, map[string]interface{}{
				"type":            f.Type,
				"file":            f.File,
				"id":              f.ID,
				"url":             f.URL,
				"name":            f.Name,
				"status":          f.Status,
				"size":            f.Size,
				"error":           f.Error,
				"itemId":          f.ItemID,
				"media":           f.Media,
				"ref_user_msg_id": userMsgID,
			})
		}
	}

	// 当使用 -tools 模型时，自动注入内置工具（客户端自带工具优先）
	if model.IsToolsModel(modelName) {
		clientToolNames := make(map[string]bool)
		for _, t := range tools {
			clientToolNames[t.Function.Name] = true
		}
		for _, bt := range builtintools.GetBuiltinTools() {
			if !clientToolNames[bt.Function.Name] {
				tools = append(tools, bt)
			}
		}
	}

	var upstreamMessages []map[string]interface{}
	hasPromptTools := len(tools) > 0

	// 提取 system 消息并转为 user+assistant 对注入对话开头
	// z.ai 会忽略 system 角色消息
	var systemTexts []string
	var nonSystemMessages []model.Message
	for _, msg := range messages {
		if msg.Role == "system" {
			text, _ := msg.ParseContent()
			if text != "" {
				systemTexts = append(systemTexts, text)
			}
		} else {
			nonSystemMessages = append(nonSystemMessages, msg)
		}
	}

	for _, msg := range nonSystemMessages {
		if hasPromptTools {
			// prompt 注入模式：将 tool_calls / tool 结果转为纯文本
			if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
				text, _ := msg.ParseContent()
				callText := builtintools.ConvertToolCallToText(msg.ToolCalls)
				if text != "" {
					text = text + "\n" + callText
				} else {
					text = callText
				}
				upstreamMessages = append(upstreamMessages, map[string]interface{}{
					"role":    "assistant",
					"content": text,
				})
				continue
			}
			if msg.Role == "tool" {
				text, _ := msg.ParseContent()
				upstreamMessages = append(upstreamMessages, map[string]interface{}{
					"role":    "user",
					"content": builtintools.ConvertToolResultToText(msg.ToolCallID, text),
				})
				continue
			}
		}
		upstreamMessages = append(upstreamMessages, msg.ToUpstreamMessage(urlToFileID))
	}

	// 工具注入：通过 user+assistant 对话注入工具定义
	// z.ai 会忽略 system 角色消息，因此使用 user/assistant 模拟注入
	if len(tools) > 0 {
		toolSystemPrompt := builtintools.BuildToolSystemPrompt(tools, toolChoice)
		if toolSystemPrompt != "" {
			logger.LogDebug("[ToolPrompt] Injecting tool system prompt (%d bytes, %d tools)", len(toolSystemPrompt), len(tools))
			userPromptMsg := map[string]interface{}{
				"role":    "user",
				"content": toolSystemPrompt,
			}
			assistantAckMsg := map[string]interface{}{
				"role":    "assistant",
				"content": "好的，我已了解可用工具。当需要使用工具时，我会直接输出 <tool_call> 标签进行调用。",
			}
			upstreamMessages = append([]map[string]interface{}{userPromptMsg, assistantAckMsg}, upstreamMessages...)
		}
	}

	// system 消息注入：通过 user+assistant 对注入对话开头
	if len(systemTexts) > 0 {
		combinedSystem := strings.Join(systemTexts, "\n\n")
		logger.LogDebug("[System] Injecting system message as user+assistant pair (%d bytes)", len(combinedSystem))
		systemUserMsg := map[string]interface{}{
			"role":    "user",
			"content": "[System Instructions]\n" + combinedSystem,
		}
		systemAssistantMsg := map[string]interface{}{
			"role":    "assistant",
			"content": "Understood. I will follow these instructions.",
		}
		upstreamMessages = append([]map[string]interface{}{systemUserMsg, systemAssistantMsg}, upstreamMessages...)
	}

	body := map[string]interface{}{
		"stream":           true,
		"model":            targetModel,
		"messages":         upstreamMessages,
		"signature_prompt": latestUserContent,
		"params":           map[string]interface{}{},
		"extra":            map[string]interface{}{},
		"variables":        map[string]interface{}{},
		"features": map[string]interface{}{
			"image_generation": false,
			"web_search":       false,
			"auto_web_search":  autoWebSearch,
			"preview_mode":     false,
			"flags":            []string{},
			"enable_thinking":  enableThinking,
		},
		"chat_id":                        chatID,
		"id":                             assistantMsgID,
		"current_user_message_id":        userMsgID,
		"current_user_message_parent_id": nil,
	}

	if len(mcpServers) > 0 {
		body["mcp_servers"] = mcpServers
	}

	if len(filesData) > 0 {
		body["files"] = filesData
		body["current_user_message_id"] = userMsgID
	}

	bodyBytes, _ := json.Marshal(body)

	// Debug: log the messages being sent
	if len(tools) > 0 {
		for i, msg := range upstreamMessages {
			role, _ := msg["role"].(string)
			content, _ := msg["content"].(string)
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			logger.LogDebug("[ToolPrompt] msg[%d] role=%s content=%s", i, role, content)
		}
	}

	req, err := http.NewRequest("POST", requestURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("X-FE-Version", version.GetFeVersion())
	req.Header.Set("X-Signature", signature)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Origin", "https://chat.z.ai")
	req.Header.Set("Referer", fmt.Sprintf("https://chat.z.ai/c/%s", chatID))
	req.Header.Set("User-Agent", userAgent)

	client := proxy.GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}

	return resp, targetModel, nil
}
