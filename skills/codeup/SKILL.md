---
name: codeup
description: 用 codeup CLI 操作阿里云云效（Codeup / Flow）：为当前分支创建合并请求（MR），查询代码库，触发流水线运行、手动执行流水线任务（如人工卡点的发布节点）并跟踪任务日志。前提是当前 git 仓库托管在 codeup.aliyun.com。只要用户在这类仓库里提到提 MR、合并请求、CR、发起评审、跑流水线、构建、部署、发布、看构建日志或云效，即使没提 codeup 也要使用本 skill。仓库托管在 GitHub/GitLab 等其他平台时不适用。
metadata:
  requires:
    bins: ["codeup", "git"]
  cliHelp: "codeup --help"
---

# Codeup CLI

`codeup` 是阿里云云效的命令行工具，源码在 github.com/postdare/codeup-cli。命令和参数以 `codeup <command> --help` 的实际输出为准；本文与 CLI 输出不一致时，按 CLI 执行。

## 第一步：确认仓库托管在 Codeup 并取得上下文

在仓库目录下运行本 skill 自带的脚本（路径相对于本 SKILL.md 所在目录）：

```bash
<skill-dir>/scripts/repo-info.sh          # 默认看 origin；其他远程名作为第一个参数传入
```

输出示例：

```
org_id=<组织 ID>
repo_path=<组织 ID>/<分组>/<代码库>
current_branch=feat/login
default_branch=main
upstream=origin/feat/login      # 为空表示当前分支从未推送
unpushed_commits=2              # 本地领先远程的提交数
repo_id=<代码库数字 ID>
web_url=https://codeup.aliyun.com/<组织 ID>/<分组>/<代码库>
```

- **退出码 2**：远程不是 Codeup，本 skill 不适用，停止使用（例如 GitHub 仓库改用 `gh`）。
- **退出码 1**：通常是 `codeup` 未安装或未配置，见下文「环境准备」。

**每条 codeup 命令都要带 `--org-id <org_id>`**。原因：用户的全局配置里可能是另一个组织的 ID（很多人属于多个组织），组织不匹配时接口一律返回 404 `There is not project with path`，看起来像代码库不存在，其实只是组织不对。`org_id` 就是远程 URL 路径的第一段。

代码库参数优先用数字 `repo_id`。用全路径时必须带组织前缀（`<org_id>/group/repo`），省略前缀会 404。

## 环境准备

- `codeup --version` 失败，说明 CLI 未安装。安装方式：`brew tap postdare/tap && brew install --cask codeup`，或 `go install github.com/postdare/codeup-cli/cmd/codeup@latest`。
- `codeup config get` 查看当前生效的配置（令牌会脱敏显示）。优先级：命令行参数 > 环境变量（`CODEUP_DOMAIN` / `CODEUP_TOKEN` / `CODEUP_ORG_ID`）> `~/.config/codeup/config.json`。
- 缺 domain 或 token 时，问用户要个人访问令牌，接入点一般是 `openapi-rdc.aliyuncs.com`。执行 `codeup config set ...` 写配置前先征得用户同意；令牌不要回显到输出或摘要里。

## 创建合并请求

1. **确定目标分支**，按以下顺序：用户明确指定的分支 > 项目文档（CLAUDE.md / AGENTS.md / README / CONTRIBUTING）约定的合入分支，例如「MR 合到 develop」> 脚本输出的 `default_branch`。拿不准就问用户；合错分支比多问一句代价大得多。
2. **确保源分支已推送**：`upstream` 为空或 `unpushed_commits` > 0 时，先执行 `git push -u origin <current_branch>`，否则接口找不到源分支，或 MR 缺少最新提交。不要 force push。
3. **写标题和描述**：阅读 `git log --no-merges <target>..HEAD` 和 `git diff --stat <target>...HEAD`，概括改了什么、为什么改。语言和风格跟随仓库已有的提交信息（中文仓库就写中文）。标题不超过 256 字符。
4. **创建**（数字 `repo_id` 会自动作为源库和目标库 ID）：

   ```bash
   codeup mr create --org-id <org_id> --repo <repo_id> \
       --source <current_branch> --target <target> \
       --title "<标题>" \
       --description "$(cat <<'EOF'
   <多行描述>
   EOF
   )" --json
   ```

5. **汇报**：从 JSON 中取 `localId`（MR 编号，显示为 `!<localId>`）、`status`，以及 `detailUrl`（为空时用 `webUrl`），把链接交给用户。

可选参数：
- `--reviewer <userId>`：可重复或用逗号分隔。只接受云效用户 ID，不接受姓名；CLI 无法按姓名查用户，用户只给了名字时，请他提供 ID，或先不加评审人。
- `--work-items <id1,id2>`：关联工作项。
- `--ai-review`：触发 AI 评审。
- 跨库 MR（例如从 fork 提交）：`--repo` 填目标库，并显式传 `--source-project-id` 和 `--target-project-id`。

只在用户要求时创建 MR。MR 会通知评审人，也会触发 MR 流水线，别人都能看到。

## 流水线

层级关系：流水线（`--pipeline`）→ 运行实例（`--run`）→ 阶段 → 任务（`--job`）。

CLI 不能列出流水线。流水线 ID 从用户、项目文档或流水线页面 URL 的 `pipelines/<数字>` 段获得，查不到就问用户。

```bash
# 最近的运行记录（可加 --status FAIL|SUCCESS|RUNNING、--per-page，最大 30）
codeup pipeline run list --org-id <org_id> --pipeline <pid>

# 运行详情：各阶段任务的 ID、状态和名称，代码源仓库地址，运行变量
codeup pipeline run get --org-id <org_id> --pipeline <pid> --run <rid>

# 触发一次新运行；不带参数时使用流水线默认配置
codeup pipeline run create --org-id <org_id> --pipeline <pid> \
    [--branch <代码源仓库地址>=<分支>] [--env key=value] [--wait]

# 手动运行某个任务（人工卡点、手动触发的发布节点）
codeup pipeline job start --org-id <org_id> --pipeline <pid> --run <rid> --job <jid>

# 任务日志：默认持续输出，直到任务结束
codeup pipeline job log --org-id <org_id> --pipeline <pid> --run <rid> --job <jid> [--follow=false]
```

要点：
- **用当前分支跑流水线**：`--branch` 左边的仓库地址必须与流水线代码源配置中的地址完全一致。先对最近一次运行执行 `run get`，从「代码源」一节复制地址，再拼上 `=<current_branch>`。分支必须已经推送。
- **覆盖运行变量**：可用的 key 列在 `run get` 的「运行变量」一节中。更复杂的参数用 `--params '<json>'` 直接传入。
- **会阻塞的命令**：`job log`（默认跟随）和 `run create --wait` 会一直轮询到任务或运行结束，可能持续几十分钟。在后台运行，或者只用 `--follow=false` 取一次当前日志。日志很长时重定向到文件，再用 tail/grep 查看，不要整段读进上下文。
- **退出码**：最终状态不是 `SUCCESS` 时，`job log` 和 `run create --wait` 以非零码退出。日志写 stdout，进度和最终状态写 stderr。
- **排查失败**：先 `run get` 找到状态为失败的任务，再用 `job log --follow=false` 读取日志末尾，定位报错。
- `run create`、`job start` 会真实构建或部署，只在用户明确要求时执行。执行 `job start` 前，先用 `run get` 确认任务名称，并向用户复述要运行的节点，尤其是名称里带「生产」「prod」「发布」的节点。

## 代码库查询

```bash
codeup repo get <repo_id 或 org_id/group/repo> --org-id <org_id> [--json]
codeup repo list --org-id <org_id> [--search <关键字>] [--no-archived] [--per-page 50] [--json]
```

## CLI 暂不支持的操作

当前支持的命令：`config set/get`、`mr create`、`repo list/get`、`pipeline run create/list/get`、`pipeline job start/log`。MR 列表/合并/关闭/评论、分支和标签管理、列出流水线、按姓名查用户等目前**都不支持**。

遇到这类需求时，先用 `codeup --help` 看新版本是否已经加入该功能。如果仍不支持，如实告诉用户，并给出对应的网页入口（例如代码库的 `web_url`）。不要编造子命令或参数。

## 常见错误

| 现象 | 原因与处理 |
| --- | --- |
| HTTP 404 `There is not project with path` | `--org-id` 与代码库所在组织不一致，或全路径缺少组织前缀。用 `repo-info.sh` 输出的 `org_id` 和 `repo_id` |
| HTTP 401/403 | 令牌无效、过期或权限不足。请用户检查或更换个人访问令牌 |
| `未配置服务接入点` / `未配置个人访问令牌` | 见上文「环境准备」 |
| `list repositories 需要配置 org-id` | 加 `--org-id` |
| `--repo 不是数字 ID 时，必须显式指定 ...` | 改用数字 `repo_id`，或补上 `--source-project-id` 和 `--target-project-id` |
| `启动流水线任务失败（接口返回 false）` | 任务当前不处于可手动运行的状态（可能已运行过，或前置阶段未完成）。先 `run get` 查看状态 |
| 其他 API 错误 | 把错误信息原样转述给用户（包含 `requestId`，方便向云效排查） |
