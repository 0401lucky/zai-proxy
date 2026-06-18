package handler

import (
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"strings"

	"zai-proxy/internal/auth"
	"zai-proxy/internal/tokenstore"
)

type tokenImportItem struct {
	Key   string `json:"key"`
	Token string `json:"token"`
}

type tokenUpdateRequest struct {
	Key   string            `json:"key"`
	Token string            `json:"token"`
	Items []tokenImportItem `json:"items"`
}

func HandleAdminPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/admin" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = adminPageTemplate.Execute(w, nil)
}

func HandleAdminTokens(w http.ResponseWriter, r *http.Request) {
	if !tokenstore.AdminEnabled() {
		http.Error(w, "Admin token API is disabled", http.StatusNotFound)
		return
	}
	if !tokenstore.IsAdminKey(extractAPIKey(r)) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, tokenstore.GetStatus())
	case http.MethodPost:
		handleTokenImport(w, r)
	case http.MethodDelete:
		handleTokenDelete(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleTokenImport(w http.ResponseWriter, r *http.Request) {
	var req tokenUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	items := req.Items
	if len(items) == 0 {
		items = []tokenImportItem{{Key: req.Key, Token: req.Token}}
	}

	for _, item := range items {
		key := strings.TrimSpace(item.Key)
		token := strings.TrimSpace(item.Token)
		if key == "" || token == "" {
			http.Error(w, "Each item requires key and token", http.StatusBadRequest)
			return
		}
		if !isValidZAIToken(token) {
			http.Error(w, "Invalid z.ai token for key: "+key, http.StatusBadRequest)
			return
		}
		if err := tokenstore.SetToken(key, token); err != nil {
			if errors.Is(err, tokenstore.ErrMissingKey) || errors.Is(err, tokenstore.ErrMissingToken) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Error(w, "Failed to save token", http.StatusInternalServerError)
			return
		}
	}

	writeJSON(w, http.StatusOK, tokenstore.GetStatus())
}

func handleTokenDelete(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.URL.Query().Get("key"))
	if key == "" {
		http.Error(w, "Missing key", http.StatusBadRequest)
		return
	}
	if err := tokenstore.DeleteToken(key); err != nil {
		if errors.Is(err, tokenstore.ErrMissingKey) {
			http.Error(w, "Missing key", http.StatusBadRequest)
			return
		}
		if errors.Is(err, tokenstore.ErrTokenNotFound) {
			http.Error(w, "Token key not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to delete token", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, tokenstore.GetStatus())
}

func isValidZAIToken(token string) bool {
	payload, err := auth.DecodeJWTPayload(token)
	return err == nil && payload != nil && payload.ID != ""
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

var adminPageTemplate = template.Must(template.New("admin").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>zai-proxy token 管理</title>
  <style>
    :root { color-scheme: light dark; font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    body { margin: 0; background: #f6f7f9; color: #1d1f23; }
    main { max-width: 920px; margin: 0 auto; padding: 32px 18px 56px; }
    h1 { margin: 0 0 8px; font-size: 28px; }
    p { color: #5f6673; line-height: 1.6; }
    section { background: #fff; border: 1px solid #e2e5ea; border-radius: 8px; padding: 18px; margin-top: 18px; }
    label { display: block; font-weight: 650; margin: 14px 0 8px; }
    input, textarea { box-sizing: border-box; width: 100%; border: 1px solid #cfd5df; border-radius: 6px; padding: 10px 12px; font: inherit; background: #fff; color: #1d1f23; }
    textarea { min-height: 180px; resize: vertical; font-family: ui-monospace, SFMono-Regular, Consolas, monospace; }
    button { border: 0; border-radius: 6px; padding: 10px 14px; font: inherit; font-weight: 650; color: #fff; background: #155eef; cursor: pointer; }
    button.secondary { background: #4b5565; }
    button.danger { background: #c4320a; }
    button:disabled { opacity: .55; cursor: not-allowed; }
    .row { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
    .hint { font-size: 13px; color: #6b7280; }
    .status { white-space: pre-wrap; font-family: ui-monospace, SFMono-Regular, Consolas, monospace; background: #101828; color: #d1fadf; padding: 12px; border-radius: 6px; overflow-x: auto; }
    table { width: 100%; border-collapse: collapse; margin-top: 12px; }
    th, td { border-bottom: 1px solid #e5e7eb; text-align: left; padding: 10px 8px; font-size: 14px; }
    @media (prefers-color-scheme: dark) {
      body { background: #111318; color: #f3f4f6; }
      section { background: #191c22; border-color: #2e3440; }
      input, textarea { background: #111318; border-color: #3b4351; color: #f3f4f6; }
      p, .hint { color: #a9b0bd; }
      th, td { border-color: #2e3440; }
    }
  </style>
</head>
<body>
<main>
  <h1>zai-proxy token 管理</h1>
  <p>在这里导入多个账号的 z.ai token。客户端统一使用环境变量 POOL_API_KEY，请求会自动从账号池轮询选择账号。</p>

  <section>
    <label for="adminKey">管理密钥 ADMIN_API_KEY</label>
    <input id="adminKey" type="password" autocomplete="current-password" placeholder="输入 Zeabur 环境变量 ADMIN_API_KEY">
    <p class="hint">管理密钥只保存在当前浏览器的 localStorage，不会发送给除本服务以外的地方。</p>
    <div class="row">
      <button onclick="saveAdminKey()">保存管理密钥</button>
      <button class="secondary" onclick="loadTokens()">刷新列表</button>
    </div>
  </section>

  <section>
    <label for="bulk">批量导入</label>
    <textarea id="bulk" spellcheck="false" placeholder="每行一个：账号名=z.ai token&#10;alice=eyJhbGciOi...&#10;bob=eyJhbGciOi..."></textarea>
    <p class="hint">账号名只是后台标识；客户端默认统一填写 POOL_API_KEY。右侧是对应账号从 chat.z.ai Cookie 中复制的 token。</p>
    <button onclick="importTokens()">导入 / 覆盖</button>
  </section>

  <section>
    <h2>已导入账号</h2>
    <div id="tokens"></div>
  </section>

  <section>
    <h2>结果</h2>
    <div id="status" class="status">等待操作</div>
  </section>
</main>

<script>
const adminInput = document.getElementById('adminKey');
const statusBox = document.getElementById('status');
const tokenBox = document.getElementById('tokens');
adminInput.value = localStorage.getItem('zai_proxy_admin_key') || '';

function adminKey() {
  return adminInput.value.trim();
}

function saveAdminKey() {
  localStorage.setItem('zai_proxy_admin_key', adminKey());
  setStatus('管理密钥已保存到当前浏览器。');
}

function headers() {
  return {
    'Authorization': 'Bearer ' + adminKey(),
    'Content-Type': 'application/json'
  };
}

function setStatus(value) {
  statusBox.textContent = typeof value === 'string' ? value : JSON.stringify(value, null, 2);
}

async function request(path, options = {}) {
  const res = await fetch(path, {
    ...options,
    headers: { ...headers(), ...(options.headers || {}) }
  });
  const text = await res.text();
  let data = text;
  try { data = text ? JSON.parse(text) : null; } catch (_) {}
  if (!res.ok) throw new Error(typeof data === 'string' ? data : JSON.stringify(data));
  return data;
}

function parseBulk() {
  return document.getElementById('bulk').value
    .split(/\r?\n/)
    .map(line => line.trim())
    .filter(Boolean)
    .map(line => {
      const index = line.indexOf('=');
      if (index <= 0) throw new Error('格式错误：' + line);
      return { key: line.slice(0, index).trim(), token: line.slice(index + 1).trim() };
    });
}

async function importTokens() {
  try {
    const items = parseBulk();
    const data = await request('/admin/tokens', {
      method: 'POST',
      body: JSON.stringify({ items })
    });
    setStatus(data);
    renderTokens(data.tokens || []);
  } catch (err) {
    setStatus('导入失败：' + err.message);
  }
}

async function loadTokens() {
  try {
    const data = await request('/admin/tokens');
    setStatus(data);
    renderTokens(data.tokens || []);
  } catch (err) {
    setStatus('刷新失败：' + err.message);
  }
}

async function deleteToken(key) {
  if (!confirm('删除代理 key：' + key + '？')) return;
  try {
    const data = await request('/admin/tokens?key=' + encodeURIComponent(key), { method: 'DELETE' });
    setStatus(data);
    renderTokens(data.tokens || []);
  } catch (err) {
    setStatus('删除失败：' + err.message);
  }
}

function renderTokens(tokens) {
  if (!tokens.length) {
    tokenBox.innerHTML = '<p class="hint">暂无导入账号。</p>';
    return;
  }
  tokenBox.innerHTML = '<table><thead><tr><th>账号名</th><th>来源</th><th>token 预览</th><th></th></tr></thead><tbody>' +
    tokens.map(item => '<tr><td>' + escapeHtml(item.key) + '</td><td>' + escapeHtml(item.source || '') + '</td><td>' + escapeHtml(item.token_preview || '') + '</td><td><button class="danger" onclick="deleteToken(\'' + escapeAttr(item.key) + '\')">删除</button></td></tr>').join('') +
    '</tbody></table>';
}

function escapeHtml(value) {
  return String(value).replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]));
}

function escapeAttr(value) {
  return String(value).replace(/\\/g, '\\\\').replace(/'/g, "\\'");
}
</script>
</body>
</html>`))
