# Zeabur 部署教程

本文档面向 Zeabur Dockerfile 部署，适用于本项目当前版本。

## 1. 部署方式

1. 打开 Zeabur 控制台，新建 Project。
2. 选择 GitHub 仓库 `zai-proxy`。
3. 部署类型选择 Dockerfile。
4. 分支选择 `main`。
5. 部署完成后，在服务设置里确认健康检查路径为 `/healthz`。

## 2. 端口

项目会读取 Zeabur 注入的 `PORT` 环境变量。

建议显式设置：

```env
PORT=8000
```

Dockerfile 已经声明：

```dockerfile
EXPOSE 8000
```

如果 Zeabur 自动注入了其他 `PORT`，程序也会按该端口监听。

## 3. 环境变量

最小配置：

```env
PORT=8000
LOG_LEVEL=info
ADMIN_API_KEY=换成你自己的管理密钥
ZAI_TOKEN_MAP_FILE=/data/zai_tokens.json
```

完整可选配置：

```env
PORT=8000
LOG_LEVEL=info
ZAI_UPSTREAM_BASE_URL=https://chat.z.ai
ZAI_CHAT_ENDPOINT_PATH=/api/v2/chat/completions
ADMIN_API_KEY=换成你自己的管理密钥
ZAI_TOKEN_MAP_FILE=/data/zai_tokens.json
ZAI_TOKEN_MAP=alice=alice账号token,bob=bob账号token
```

说明：

- `PORT`：服务监听端口，默认 `8000`。
- `LOG_LEVEL`：日志等级，可选 `debug`、`info`、`warn`、`error`。
- `ZAI_UPSTREAM_BASE_URL`：z.ai 上游地址，通常不用改。
- `ZAI_CHAT_ENDPOINT_PATH`：聊天补全上游路径，通常不用改。
- `ADMIN_API_KEY`：访问 `/admin` token 管理网页和 API 的管理密钥。
- `ZAI_TOKEN_MAP_FILE`：多账号 token 持久化文件路径。
- `ZAI_TOKEN_MAP`：可选初始账号映射，格式是 `代理key=z.ai token`。

## 4. 卷挂载

如果只通过 `ZAI_TOKEN_MAP` 环境变量写死账号，不需要配置卷。

如果你希望在 `/admin` 网页导入和更新 token，并且服务重启后保留，
建议在 Zeabur 添加一个持久化目录卷：

```text
容器路径：/data
```

然后设置：

```env
ZAI_TOKEN_MAP_FILE=/data/zai_tokens.json
```

`/data/zai_tokens.json` 会由服务自动创建和更新。

## 5. 代理池卷（可选）

只有在你需要让服务通过 SOCKS5 代理访问 z.ai 时，才需要额外挂载
`proxies.txt` 到容器的 `/app/proxies.txt`。

文件内容格式：

```text
ip:port
ip:port:username:password
```

Zeabur 上的建议：

1. 不需要代理：不要配置 `/app/proxies.txt`。
2. 需要代理：创建一个文件型挂载，容器路径填写 `/app/proxies.txt`。
3. 不要把目录挂载到 `/app/proxies.txt`，它必须是文件。

## 6. 健康检查

推荐健康检查路径：

```text
/healthz
```

预期响应：

```json
{"status":"ok"}
```

`/v1/models` 也无需鉴权，可作为备用健康检查路径。

## 7. 客户端配置

OpenAI 兼容客户端：

```text
Base URL: https://你的-zeabur域名/v1
API Key: 你在 /admin 里设置的代理 key
Model: glm-5.2
```

Anthropic 兼容客户端：

```text
Base URL: https://你的-zeabur域名
API Key: 你在 /admin 里设置的代理 key
Model: claude-sonnet-4-6
```

如果你不使用网页导入，也可以继续让客户端直接填 z.ai token。
推荐 Zeabur 部署使用 `/admin` 导入方式，这样不用把 z.ai token
分发到每个客户端。

## 8. Token 获取与热更新

推荐使用个人 token：

1. 登录 https://chat.z.ai
2. 打开浏览器开发者工具。
3. 进入 Application 或 Storage。
4. 在 Cookies 中找到 `token`。
5. 将该值作为客户端 API Key 使用。

`free` 匿名令牌仍可触发 guest token 获取，但截至 2026-06-18，
chat.z.ai 可能要求前端验证码。Zeabur 后端无法交互完成滑块验证，
因此生产使用不建议依赖 `free`。

打开网页导入多个账号：

```text
https://你的-zeabur域名/admin
```

网页里按行导入：

```text
alice=alice账号的_z.ai_token
bob=bob账号的_z.ai_token
```

客户端 API key 填 `alice` 就会使用 alice 账号，填 `bob` 就会使用 bob 账号。

也可以用 API 批量导入：

```bash
curl https://你的-zeabur域名/admin/tokens \
  -X POST \
  -H "Authorization: Bearer 你的_ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"items":[{"key":"alice","token":"alice账号token"},{"key":"bob","token":"bob账号token"}]}'
```

说明：当前 `chat.z.ai` 前端没有发现可用的 refresh token 接口。
这里实现的是“网页导入 + 热更新 + 文件持久化”，不是自动刷新网页登录态。

## 9. 验证命令

部署完成后，先检查服务健康：

```bash
curl https://你的-zeabur域名/healthz
```

检查模型列表：

```bash
curl https://你的-zeabur域名/v1/models
```

发送一次 OpenAI 兼容请求：

```bash
curl https://你的-zeabur域名/v1/chat/completions \
  -H "Authorization: Bearer alice" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "glm-5.2",
    "messages": [{"role": "user", "content": "hello"}],
    "stream": false
  }'
```
