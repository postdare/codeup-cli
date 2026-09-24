#!/usr/bin/env bash
# 输出当前 git 仓库对应的 Codeup 代码库上下文（key=value 每行一项）。
# 用法: repo-info.sh [remote]      remote 默认 origin
# 退出码: 0 成功；2 不是托管在 Codeup 的仓库；1 其他错误（如 codeup 未安装/未配置）。
set -euo pipefail

remote="${1:-origin}"

url=$(git remote get-url "$remote" 2>/dev/null) || {
  echo "当前目录不是 git 仓库，或没有名为 $remote 的远程" >&2
  exit 2
}
case "$url" in
  *codeup.aliyun.com*) ;;
  *)
    echo "远程 $remote 不是 Codeup 地址: $url" >&2
    exit 2
    ;;
esac

# git@codeup.aliyun.com:<org>/<group>/<repo>.git 或 https://codeup.aliyun.com/<org>/<group>/<repo>.git
repo_path=$(printf '%s' "$url" | sed -E 's#^[a-z+]+://[^/]+/##; s#^[^@/]+@[^:]+:##; s#\.git$##; s#/$##')
org_id=${repo_path%%/*}

branch=$(git symbolic-ref --short -q HEAD || echo "")

default_branch=$(git symbolic-ref --short -q "refs/remotes/$remote/HEAD" 2>/dev/null | sed "s#^$remote/##" || true)
if [ -z "$default_branch" ]; then
  default_branch=$(git ls-remote --symref "$remote" HEAD 2>/dev/null | sed -n 's#^ref: refs/heads/\(.*\)[[:space:]]HEAD$#\1#p' || true)
fi

# 当前分支是否已推送：upstream 为空表示从未推送；unpushed 为本地领先远程的提交数。
upstream=""
unpushed=""
if [ -n "$branch" ]; then
  if upstream=$(git rev-parse --abbrev-ref -q "$branch@{u}" 2>/dev/null); then
    unpushed=$(git rev-list --count "$upstream..HEAD")
  elif git rev-parse --verify -q "refs/remotes/$remote/$branch" >/dev/null; then
    upstream="$remote/$branch"
    unpushed=$(git rev-list --count "$upstream..HEAD")
  else
    upstream=""
  fi
fi

echo "org_id=$org_id"
echo "repo_path=$repo_path"
echo "current_branch=$branch"
echo "default_branch=$default_branch"
echo "upstream=$upstream"
echo "unpushed_commits=$unpushed"

# 数字 ID 需要调接口查询；--org-id 必须是代码库所在组织，否则接口返回 404。
if ! command -v codeup >/dev/null 2>&1; then
  echo "未找到 codeup 命令，无法查询代码库 ID" >&2
  exit 1
fi
json=$(codeup repo get "$repo_path" --org-id "$org_id" --json) || exit 1
if command -v jq >/dev/null 2>&1; then
  repo_id=$(printf '%s' "$json" | jq -r '.id')
  web_url=$(printf '%s' "$json" | jq -r '.webUrl')
else
  repo_id=$(printf '%s' "$json" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
  web_url=$(printf '%s' "$json" | python3 -c 'import json,sys; print(json.load(sys.stdin)["webUrl"])')
fi
echo "repo_id=$repo_id"
echo "web_url=$web_url"
