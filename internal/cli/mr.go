package cli

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/postdare/codeup-cli/internal/api"
)

var mrCmd = &cobra.Command{
	Use:     "mr",
	Aliases: []string{"merge-request", "cr"},
	Short:   "管理合并请求",
}

var mrCreateOpts struct {
	repo            string
	title           string
	description     string
	sourceBranch    string
	targetBranch    string
	sourceProjectID int64
	targetProjectID int64
	reviewers       []string
	workItemIDs     string
	aiReview        bool
	outputJSON      bool
}

var mrCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "创建合并请求",
	Example: `  # 数字 ID 形式的代码库（源/目标库 ID 默认取该 ID）
  codeup mr create --repo 2813489 --source feat/login --target master --title "支持登录"

  # 全路径形式的代码库，需要显式指定源/目标库 ID
  codeup mr create --repo myorg/demo-repo --source-project-id 2813489 --target-project-id 2813489 \
      --source feat/login --target master --title "支持登录" \
      --description "实现登录功能" --reviewer 62c795xxxb468af8 --ai-review`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		o := &mrCreateOpts

		// 代码库为数字 ID 时，源/目标库 ID 默认与其一致（同库分支合并的常见场景）。
		if repoID, err := strconv.ParseInt(o.repo, 10, 64); err == nil {
			if o.sourceProjectID == 0 {
				o.sourceProjectID = repoID
			}
			if o.targetProjectID == 0 {
				o.targetProjectID = repoID
			}
		}
		if o.sourceProjectID == 0 || o.targetProjectID == 0 {
			return fmt.Errorf("--repo 不是数字 ID 时，必须显式指定 --source-project-id 和 --target-project-id")
		}

		client, err := newAPIClient()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
		defer cancel()

		mr, err := client.CreateMergeRequest(ctx, o.repo, &api.CreateMergeRequestOptions{
			Title:              o.title,
			Description:        o.description,
			SourceBranch:       o.sourceBranch,
			SourceProjectID:    o.sourceProjectID,
			TargetBranch:       o.targetBranch,
			TargetProjectID:    o.targetProjectID,
			ReviewerUserIDs:    o.reviewers,
			TriggerAIReviewRun: o.aiReview,
			WorkItemIDs:        o.workItemIDs,
		})
		if err != nil {
			return err
		}

		if o.outputJSON {
			return printJSON(mr)
		}

		fmt.Printf("✓ 合并请求创建成功 !%d\n", mr.LocalID)
		fmt.Printf("  标题:   %s\n", mr.Title)
		fmt.Printf("  分支:   %s -> %s\n", mr.SourceBranch, mr.TargetBranch)
		fmt.Printf("  状态:   %s\n", mr.Status)
		if len(mr.Reviewers) > 0 {
			fmt.Print("  评审人: ")
			for i, r := range mr.Reviewers {
				if i > 0 {
					fmt.Print(", ")
				}
				fmt.Print(r.Name)
			}
			fmt.Println()
		}
		if url := mr.DetailURL; url != "" {
			fmt.Printf("  链接:   %s\n", url)
		} else if mr.WebURL != "" {
			fmt.Printf("  链接:   %s\n", mr.WebURL)
		}
		return nil
	},
}

func init() {
	f := mrCreateCmd.Flags()
	f.StringVarP(&mrCreateOpts.repo, "repo", "r", "", "代码库 ID 或全路径（如 myorg/demo-repo）")
	f.StringVarP(&mrCreateOpts.title, "title", "t", "", "标题（必填，不超过 256 字符）")
	f.StringVarP(&mrCreateOpts.description, "description", "d", "", "描述（不超过 10000 字符）")
	f.StringVarP(&mrCreateOpts.sourceBranch, "source", "s", "", "源分支（必填）")
	f.StringVar(&mrCreateOpts.targetBranch, "target", "", "目标分支（必填）")
	f.Int64Var(&mrCreateOpts.sourceProjectID, "source-project-id", 0, "源库 ID（--repo 为数字时默认取其值）")
	f.Int64Var(&mrCreateOpts.targetProjectID, "target-project-id", 0, "目标库 ID（--repo 为数字时默认取其值）")
	f.StringSliceVar(&mrCreateOpts.reviewers, "reviewer", nil, "评审人用户 ID（可重复或逗号分隔）")
	f.StringVar(&mrCreateOpts.workItemIDs, "work-items", "", "关联工作项 ID 列表（逗号分隔）")
	f.BoolVar(&mrCreateOpts.aiReview, "ai-review", false, "触发 AI 评审")
	f.BoolVar(&mrCreateOpts.outputJSON, "json", false, "以 JSON 输出完整返回结果")

	_ = mrCreateCmd.MarkFlagRequired("repo")
	_ = mrCreateCmd.MarkFlagRequired("title")
	_ = mrCreateCmd.MarkFlagRequired("source")
	_ = mrCreateCmd.MarkFlagRequired("target")

	mrCmd.AddCommand(mrCreateCmd)
	rootCmd.AddCommand(mrCmd)
}
