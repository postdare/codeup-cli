package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/postdare/codeup-cli/internal/api"
)

var repoCmd = &cobra.Command{
	Use:     "repo",
	Aliases: []string{"repository"},
	Short:   "管理代码库",
}

var repoListOpts struct {
	search     string
	page       int
	perPage    int
	noArchived bool
	outputJSON bool
}

var repoListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出代码库",
	Example: `  codeup repo list
  codeup repo list --search lingbo
  codeup repo list --per-page 50 --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		o := &repoListOpts

		client, err := newAPIClient()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
		defer cancel()

		var archived *bool
		if o.noArchived {
			f := false
			archived = &f
		}

		repos, err := client.ListRepositories(ctx, api.ListRepositoriesOptions{
			Search:   o.search,
			Page:     o.page,
			PerPage:  o.perPage,
			Archived: archived,
		})
		if err != nil {
			return err
		}

		if o.outputJSON {
			return printJSON(repos)
		}

		if len(repos) == 0 {
			fmt.Println("未找到代码库")
			return nil
		}

		fmt.Printf("%-10s  %-30s  %-12s  %s\n", "ID", "名称", "可见性", "最后活跃")
		fmt.Println(strings.Repeat("-", 72))
		for _, r := range repos {
			activity := r.LastActivityAt
			if len(activity) > 10 {
				activity = activity[:10]
			}
			fmt.Printf("%-10d  %-30s  %-12s  %s\n", r.ID, r.Name, r.Visibility, activity)
		}
		return nil
	},
}

var repoGetOpts struct {
	outputJSON bool
}

var repoGetCmd = &cobra.Command{
	Use:   "get <代码库ID或全路径>",
	Short: "获取代码库详情",
	Example: `  codeup repo get 5677164
  codeup repo get lingbo/lingbo-funds`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newAPIClient()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
		defer cancel()

		repo, err := client.GetRepository(ctx, args[0])
		if err != nil {
			return err
		}

		if repoGetOpts.outputJSON {
			return printJSON(repo)
		}

		fmt.Printf("ID:         %d\n", repo.ID)
		fmt.Printf("名称:       %s\n", repo.Name)
		fmt.Printf("路径:       %s\n", repo.PathWithNamespace)
		if repo.Description != "" {
			fmt.Printf("描述:       %s\n", repo.Description)
		}
		fmt.Printf("可见性:     %s\n", repo.Visibility)
		fmt.Printf("HTTP:       %s\n", repo.HttpUrlToRepo)
		fmt.Printf("SSH:        %s\n", repo.SshUrlToRepo)
		fmt.Printf("页面:       %s\n", repo.WebURL)
		fmt.Printf("最后活跃:   %s\n", repo.LastActivityAt)
		return nil
	},
}

func init() {
	fl := repoListCmd.Flags()
	fl.StringVarP(&repoListOpts.search, "search", "s", "", "按名称搜索")
	fl.IntVar(&repoListOpts.page, "page", 1, "页码")
	fl.IntVar(&repoListOpts.perPage, "per-page", 20, "每页数量（最大 100）")
	fl.BoolVar(&repoListOpts.noArchived, "no-archived", false, "排除已归档代码库")
	fl.BoolVar(&repoListOpts.outputJSON, "json", false, "以 JSON 输出")

	repoGetCmd.Flags().BoolVar(&repoGetOpts.outputJSON, "json", false, "以 JSON 输出")

	repoCmd.AddCommand(repoListCmd, repoGetCmd)
	rootCmd.AddCommand(repoCmd)
}
