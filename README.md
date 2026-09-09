# codeup-cli

阿里云云效（Codeup）命令行工具，使用 Go 开发。当前版本支持**创建合并请求**、**手动运行流水线任务并持续查看任务日志**。

## 安装

### Homebrew（macOS，推荐）

```bash
brew tap postdare/tap
brew install --cask codeup
```

安装的是 GitHub Release 上的预编译二进制，无需本地 Go 环境。

### go install（需 Go 1.24+）

```bash
go install github.com/postdare/codeup-cli/cmd/codeup@latest
```

安装后命令名为 `codeup`（确保 `$GOPATH/bin` 在 PATH 中）。

### 使用发布包（Windows 或无 Go 环境）

从发布包中取对应平台的压缩包解压即可：

| 平台 | 包名 |
| --- | --- |
| macOS（Apple Silicon / M 系列） | `codeup-<版本>-darwin-arm64.tar.gz` |
| macOS（Intel） | `codeup-<版本>-darwin-amd64.tar.gz` |
| Windows（64 位） | `codeup-<版本>-windows-amd64.zip` |
| Linux | `codeup-<版本>-linux-amd64.tar.gz` |

**macOS**：

```bash
tar -xzf codeup-*-darwin-arm64.tar.gz
mkdir -p ~/.local/bin && mv codeup-*/codeup ~/.local/bin/
# 若从浏览器/聊天工具下载，macOS 会隔离未签名二进制，需放行一次：
xattr -d com.apple.quarantine ~/.local/bin/codeup 2>/dev/null || true
# 确保 ~/.local/bin 在 PATH 中（zsh）：
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc
```

**Windows**：解压 zip，把 `codeup.exe` 放到一个固定目录（如 `C:\Tools\codeup\`），然后在「系统设置 → 环境变量」给 `Path` 追加该目录；或直接用 PowerShell：

```powershell
[Environment]::SetEnvironmentVariable("Path", $env:Path + ";C:\Tools\codeup", "User")
```

重开终端后运行 `codeup --help` 验证。

### 从源码构建

```bash
make build     # 编译 ./codeup
make install   # 安装到 $GOPATH/bin
make release   # 在 dist/ 下生成 mac/Windows/Linux 各平台压缩包
```

## 配置

配置优先级：**命令行参数 > 环境变量 > 配置文件**（`~/.config/codeup/config.json`）。

| 配置项 | 环境变量 | 说明 |
| --- | --- | --- |
| `domain` | `CODEUP_DOMAIN` | [服务接入点](https://help.aliyun.com/zh/yunxiao/developer-reference/service-access-point-domain) |
| `token` | `CODEUP_TOKEN` | [个人访问令牌](https://help.aliyun.com/zh/yunxiao/developer-reference/obtain-personal-access-token) |
| `org-id` | `CODEUP_ORG_ID` | 组织 ID，**仅中心版需要**；留空则按 Region 版调用 |

```bash
codeup config set domain openapi-rdc.aliyuncs.com
codeup config set token pt-xxxx
codeup config set org-id 60d54f3daccf2bbd6659f3ad   # 仅中心版
codeup config get                                    # 查看当前生效配置（令牌脱敏显示）
```

## 创建合并请求

```bash
# 同库分支合并（--repo 为数字 ID 时，源/目标库 ID 自动取该 ID）
codeup mr create --repo 2813489 \
    --source feat/login --target master \
    --title "支持登录" --description "实现登录功能"

# 代码库全路径形式，需显式指定源/目标库 ID
codeup mr create --repo myorg/demo-repo \
    --source-project-id 2813489 --target-project-id 2813489 \
    --source feat/login --target master --title "支持登录"

# 指定评审人、关联工作项、触发 AI 评审、JSON 输出
codeup mr create --repo 2813489 -s feat/login --target master -t "支持登录" \
    --reviewer 62c795xxxb468af8 --work-items 722200214032b6b31e6f1434ab \
    --ai-review --json
```

成功输出示例：

```
✓ 合并请求创建成功 !1
  标题:   支持登录
  分支:   feat/login -> master
  状态:   UNDER_REVIEW
  链接:   https://example.com/example/demo/change/1
```

## 流水线任务

```bash
# 手动运行流水线中的某个任务（如人工确认的部署节点）
codeup pipeline job start --pipeline 123 --run 1 --job 21212

# 持续输出任务日志，直到任务结束（默认行为）
codeup pipeline job log --pipeline 123 --run 1 --job 21212

# 调整轮询间隔 / 只取一次当前日志
codeup pipeline job log --pipeline 123 --run 1 --job 21212 --interval 5s
codeup pipeline job log --pipeline 123 --run 1 --job 21212 --follow=false
```

说明：

- `--run`（流水线运行实例 ID）与 `--job`（任务 ID）可分别在流水线运行记录页 URL 或 `GetPipelineRun` 接口返回中查到。
- 持续获取日志时，日志写 stdout、进度与最终状态写 stderr，可直接管道处理日志。
- 任务最终状态不是 `SUCCESS` 时命令以非零码退出，便于在脚本/CI 中判断发布成败。

## 项目结构

```
cmd/codeup/main.go         入口（go install 的目标，命令名 codeup）
internal/cli/              命令定义（cobra）
  root.go                  根命令、全局参数、版本号、配置装配
  config.go                codeup config set/get
  mr.go                    codeup mr create
  pipeline.go              codeup pipeline job start/log
internal/config/           配置文件 + 环境变量加载
internal/api/              云效 OpenAPI 客户端
  client.go                HTTP 封装、中心版/Region 版路径、错误处理
  mr.go                    创建合并请求接口与数据模型
  flow.go                  流水线任务启动 / 日志查询 / 运行实例查询接口
```

## 开发

```bash
go test ./...
go vet ./...
```
