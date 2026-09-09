package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/postdare/codeup-cli/internal/api"
)

var pipelineCmd = &cobra.Command{
	Use:     "pipeline",
	Aliases: []string{"flow"},
	Short:   "管理流水线",
}

var pipelineJobCmd = &cobra.Command{
	Use:   "job",
	Short: "管理流水线运行任务",
}

// pipelineJobFlags 是 job 子命令共用的定位参数。
type pipelineJobFlags struct {
	pipelineID string
	runID      string
	jobID      string
}

func (f *pipelineJobFlags) register(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.StringVar(&f.pipelineID, "pipeline", "", "流水线 ID（必填）")
	flags.StringVar(&f.runID, "run", "", "流水线运行实例 ID（必填）")
	flags.StringVar(&f.jobID, "job", "", "任务 ID（必填）")
	_ = cmd.MarkFlagRequired("pipeline")
	_ = cmd.MarkFlagRequired("run")
	_ = cmd.MarkFlagRequired("job")
}

var jobStartOpts struct {
	pipelineJobFlags
	outputJSON bool
}

var jobStartCmd = &cobra.Command{
	Use:     "start",
	Short:   "手动运行流水线任务",
	Example: `  codeup pipeline job start --pipeline 123 --run 1 --job 21212`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		o := &jobStartOpts

		client, err := newAPIClient()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
		defer cancel()

		if err := client.ExecutePipelineJobRun(ctx, o.pipelineID, o.runID, o.jobID); err != nil {
			return err
		}

		if o.outputJSON {
			return printJSON(map[string]string{
				"pipelineId":    o.pipelineID,
				"pipelineRunId": o.runID,
				"jobId":         o.jobID,
				"status":        "started",
			})
		}
		fmt.Printf("✓ 任务 %s 已启动（流水线 %s，运行实例 %s）\n", o.jobID, o.pipelineID, o.runID)
		fmt.Printf("  可使用 `codeup pipeline job log --pipeline %s --run %s --job %s` 持续查看日志\n",
			o.pipelineID, o.runID, o.jobID)
		return nil
	},
}

var jobLogOpts struct {
	pipelineJobFlags
	follow     bool
	interval   time.Duration
	outputJSON bool
}

var jobLogCmd = &cobra.Command{
	Use:   "log",
	Short: "查询任务运行日志（默认持续获取直到任务结束）",
	Example: `  # 持续输出日志直到任务结束
  codeup pipeline job log --pipeline 123 --run 1 --job 21212

  # 只取一次当前日志
  codeup pipeline job log --pipeline 123 --run 1 --job 21212 --follow=false`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		o := &jobLogOpts

		client, err := newAPIClient()
		if err != nil {
			return err
		}

		if o.outputJSON || !o.follow {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			log, err := client.GetPipelineJobRunLog(ctx, o.pipelineID, o.runID, o.jobID)
			if err != nil {
				return err
			}
			if o.outputJSON {
				return printJSON(log)
			}
			fmt.Print(log.Content)
			return nil
		}

		return followJobLog(cmd.Context(), client, o.pipelineJobFlags, o.interval)
	},
}

// jobLogAPI 抽象 followJobLog 依赖的接口方法，便于测试。
type jobLogAPI interface {
	GetPipelineJobRunLog(ctx context.Context, pipelineID, pipelineRunID, jobID string) (*api.PipelineJobRunLog, error)
	GetPipelineJobStatus(ctx context.Context, pipelineID, pipelineRunID, jobID string) (*api.PipelineRunJob, error)
}

// followJobLog 持续轮询任务日志并增量输出，直到任务到达终态且日志取完。
// 日志内容写 stdout，进度与结果信息写 stderr，方便管道处理日志。
func followJobLog(ctx context.Context, client jobLogAPI, f pipelineJobFlags, interval time.Duration) error {
	if interval <= 0 {
		interval = 2 * time.Second
	}

	var printed string
	for {
		log, err := client.GetPipelineJobRunLog(ctx, f.pipelineID, f.runID, f.jobID)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}

		// 日志接口可能返回全量内容（content 含已打印前缀），也可能只返回增量；
		// 按前缀去重以同时兼容两种语义。
		if newContent := strings.TrimPrefix(log.Content, printed); newContent != "" {
			fmt.Print(newContent)
			if !strings.HasSuffix(newContent, "\n") {
				fmt.Println()
			}
		}
		printed = log.Content

		job, err := client.GetPipelineJobStatus(ctx, f.pipelineID, f.runID, f.jobID)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}

		if api.PipelineJobTerminal(job.Status) && !log.More {
			fmt.Fprintf(os.Stderr, "\n任务 %s（%s）已结束，状态: %s\n", f.jobID, job.Name, job.Status)
			if job.Status != "SUCCESS" {
				return fmt.Errorf("任务未成功，最终状态: %s", job.Status)
			}
			return nil
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(interval):
		}
	}
}

var runCreateOpts struct {
	pipelineID string
	envs       []string
	branches   []string
	branchMode []string
	tags       []string
	params     string
	wait       bool
	outputJSON bool
}

// runCreateParams 是 run create 的选项结构（与全局 flag 变量分离，便于测试）。
type runCreateParams struct {
	envs       []string
	branches   []string
	branchMode []string
	tags       []string
	params     string
}

var runCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "运行流水线（触发一次新的运行）",
	Example: `  # 使用默认配置运行
  codeup pipeline run create --pipeline 5251139

  # 指定运行分支、运行变量
  codeup pipeline run create --pipeline 5251139 \
      --branch https://codeup.aliyun.com/org/repo.git=master \
      --env app=lingbo-crm --env version=1.2.0

  # 触发后持续跟踪日志直到运行结束
  codeup pipeline run create --pipeline 5251139 --wait`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		o := &runCreateOpts

		client, err := newAPIClient()
		if err != nil {
			return err
		}

		params, err := buildRunParams(&runCreateParams{
			envs:       o.envs,
			branches:   o.branches,
			branchMode: o.branchMode,
			tags:       o.tags,
			params:     o.params,
		})
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
		defer cancel()

		runID, err := client.CreatePipelineRun(ctx, o.pipelineID, &api.CreatePipelineRunOptions{Params: params})
		if err != nil {
			return err
		}

		if o.outputJSON {
			return printJSON(map[string]any{"pipelineId": o.pipelineID, "pipelineRunId": runID})
		}
		fmt.Printf("✓ 流水线 %s 已触发运行，运行实例 ID: %d\n", o.pipelineID, runID)
		fmt.Printf("  可使用 `codeup pipeline run get --pipeline %s --run %d` 查看任务列表\n", o.pipelineID, runID)

		if !o.wait {
			return nil
		}
		return watchRun(cmd.Context(), client, o.pipelineID, fmt.Sprintf("%d", runID), 10*time.Second)
	},
}

// buildRunParams 把命令行参数组装为运行参数 JSON 字符串，无参数时返回空串。
func buildRunParams(o *runCreateParams) (string, error) {
	hasAny := len(o.envs) > 0 || len(o.branches) > 0 || len(o.branchMode) > 0 || len(o.tags) > 0 || o.params != ""
	if !hasAny {
		return "", nil
	}

	p := map[string]any{}
	if len(o.branchMode) > 0 {
		p["branchModeBranchs"] = o.branchMode
	}
	if kv, err := parseKVPairs(o.envs); err != nil {
		return "", fmt.Errorf("--env 参数错误: %w", err)
	} else if len(kv) > 0 {
		p["envs"] = kv
	}
	if m, err := parseURLMap(o.branches); err != nil {
		return "", fmt.Errorf("--branch 参数错误: %w", err)
	} else if len(m) > 0 {
		p["runningBranchs"] = m
	}
	if m, err := parseURLMap(o.tags); err != nil {
		return "", fmt.Errorf("--tag 参数错误: %w", err)
	} else if len(m) > 0 {
		p["runningTags"] = m
	}
	if o.params != "" {
		var extra map[string]any
		if err := json.Unmarshal([]byte(o.params), &extra); err != nil {
			return "", fmt.Errorf("--params 不是合法 JSON: %w", err)
		}
		for k, v := range extra {
			p[k] = v
		}
	}

	data, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// parseKVPairs 解析 key=value 形式的字符串列表。
func parseKVPairs(items []string) (map[string]string, error) {
	m := map[string]string{}
	for _, it := range items {
		k, v, ok := strings.Cut(it, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("%q 不是 key=value 形式", it)
		}
		m[k] = v
	}
	return m, nil
}

// parseURLMap 解析 URL=值 形式的字符串列表（runningBranchs/runningTags 的 key 为仓库地址）。
func parseURLMap(items []string) (map[string]string, error) {
	m := map[string]string{}
	for _, it := range items {
		u, v, ok := strings.Cut(it, "=")
		if !ok || u == "" || v == "" {
			return nil, fmt.Errorf("%q 不是 仓库地址=分支/tag 形式", it)
		}
		m[u] = v
	}
	return m, nil
}

// watchRun 轮询运行实例状态直到结束（简单轮询整体状态，不逐任务拉日志）。
func watchRun(ctx context.Context, client runWatcher, pipelineID, runID string, interval time.Duration) error {
	fmt.Fprintf(os.Stderr, "跟踪运行实例 %s 状态（每 %s 轮询，Ctrl+C 退出）…\n", runID, interval)
	for {
		run, err := client.GetPipelineRun(ctx, pipelineID, runID)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}

		fmt.Fprintf(os.Stderr, "  [%s] 状态: %s\n", time.Now().Format("15:04:05"), run.Status)
		if api.PipelineJobTerminal(run.Status) {
			fmt.Fprintf(os.Stderr, "\n运行实例 %s 已结束，状态: %s\n", runID, run.Status)
			if run.Status != "SUCCESS" {
				return fmt.Errorf("流水线运行未成功，最终状态: %s", run.Status)
			}
			return nil
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(interval):
		}
	}
}

// runWatcher 抽象 watchRun 依赖的接口方法，便于测试。
type runWatcher interface {
	GetPipelineRun(ctx context.Context, pipelineID, runID string) (*api.PipelineRun, error)
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "管理流水线运行实例",
}

var runListOpts struct {
	pipelineID  string
	page        int
	perPage     int
	status      string
	triggerMode int
	outputJSON  bool
}

var runListCmd = &cobra.Command{
	Use:     "list",
	Short:   "获取流水线运行实例列表",
	Example: "  codeup pipeline run list --pipeline 5251139\n  codeup pipeline run list --pipeline 5251139 --status FAIL",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		o := &runListOpts

		client, err := newAPIClient()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
		defer cancel()

		items, headers, err := client.ListPipelineRuns(ctx, o.pipelineID, api.ListPipelineRunsOptions{
			Page:        o.page,
			PerPage:     o.perPage,
			Status:      o.status,
			TriggerMode: o.triggerMode,
		})
		if err != nil {
			return err
		}

		if o.outputJSON {
			return printJSON(items)
		}
		if len(items) == 0 {
			fmt.Println("暂无运行记录")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "运行ID\t状态\t开始时间\t结束时间\t触发方式")
		for _, r := range items {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n",
				r.PipelineRunID,
				r.StatusOrUnknown(),
				formatMilli(r.StartTime),
				formatMilli(r.EndTime),
				triggerModeName(r.TriggerMode),
			)
		}
		_ = w.Flush()

		if total := headers.Get("x-total"); total != "" {
			fmt.Printf("\n共 %s 条，当前页 %s/%s\n", total, headers.Get("x-page"), headers.Get("x-total-pages"))
		}
		return nil
	},
}

var runGetOpts struct {
	pipelineID string
	runID      string
	outputJSON bool
}

var runGetCmd = &cobra.Command{
	Use:     "get",
	Short:   "获取流水线运行实例详情（含各阶段任务的状态与任务 ID）",
	Example: "  codeup pipeline run get --pipeline 5251139 --run 12",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		o := &runGetOpts

		client, err := newAPIClient()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
		defer cancel()

		run, err := client.GetPipelineRun(ctx, o.pipelineID, o.runID)
		if err != nil {
			return err
		}

		if o.outputJSON {
			return printJSON(run)
		}

		fmt.Printf("运行实例 %d（流水线 %d）状态: %s\n\n", run.PipelineRunID, run.PipelineID, run.Status)

		if len(run.Sources) > 0 {
			fmt.Println("代码源:")
			for _, s := range run.Sources {
				fmt.Printf("    %-10s %s  %s（默认分支: %s）\n", s.Type, s.Name, s.Data.Repo, s.Data.Branch)
			}
			fmt.Println()
		}
		if len(run.GlobalParams) > 0 {
			fmt.Println("运行变量（--env 可覆盖）:")
			for _, p := range run.GlobalParams {
				v := p.Value
				if p.Encrypted {
					v = "******"
				}
				fmt.Printf("    %-24s %s\n", p.Key, v)
			}
			fmt.Println()
		}

		for _, st := range run.Stages {
			fmt.Printf("[%s] %s（%s）\n", st.Index, st.Name, st.StageInfo.Status)
			for _, j := range st.StageInfo.Jobs {
				fmt.Printf("    job %-10d %-8s %s\n", j.ID, j.Status, j.Name)
			}
		}
		fmt.Println("\n提示: 使用上面的任务 ID 执行 codeup pipeline job start/log")
		return nil
	},
}

func init() {
	f := runListCmd.Flags()
	f.StringVar(&runListOpts.pipelineID, "pipeline", "", "流水线 ID（必填）")
	f.IntVar(&runListOpts.page, "page", 1, "页码")
	f.IntVar(&runListOpts.perPage, "per-page", 10, "每页条数（最大 30）")
	f.StringVar(&runListOpts.status, "status", "", "按状态过滤：FAIL / SUCCESS / RUNNING")
	f.IntVar(&runListOpts.triggerMode, "trigger-mode", 0, "按触发方式过滤：1 人工 2 定时 3 代码提交 5 流水线 6 WEBHOOK")
	f.BoolVar(&runListOpts.outputJSON, "json", false, "以 JSON 输出")
	_ = runListCmd.MarkFlagRequired("pipeline")

	g := runGetCmd.Flags()
	g.StringVar(&runGetOpts.pipelineID, "pipeline", "", "流水线 ID（必填）")
	g.StringVar(&runGetOpts.runID, "run", "", "运行实例 ID（必填）")
	g.BoolVar(&runGetOpts.outputJSON, "json", false, "以 JSON 输出")
	_ = runGetCmd.MarkFlagRequired("pipeline")
	_ = runGetCmd.MarkFlagRequired("run")

	c := runCreateCmd.Flags()
	c.StringVar(&runCreateOpts.pipelineID, "pipeline", "", "流水线 ID（必填）")
	c.StringSliceVar(&runCreateOpts.envs, "env", nil, "运行变量，key=value（可重复）")
	c.StringSliceVar(&runCreateOpts.branches, "branch", nil, "运行分支，仓库地址=分支（可重复）")
	c.StringSliceVar(&runCreateOpts.branchMode, "branch-mode", nil, "分支模式运行分支（可重复）")
	c.StringSliceVar(&runCreateOpts.tags, "tag", nil, "运行 tag，仓库地址=tag（可重复）")
	c.StringVar(&runCreateOpts.params, "params", "", "完整运行参数 JSON 字符串（与上面选项合并，优先级更高）")
	c.BoolVar(&runCreateOpts.wait, "wait", false, "触发后轮询运行状态直到结束")
	c.BoolVar(&runCreateOpts.outputJSON, "json", false, "以 JSON 输出")
	_ = runCreateCmd.MarkFlagRequired("pipeline")

	runCmd.AddCommand(runListCmd, runGetCmd, runCreateCmd)
	runCmd.AddCommand(runListCmd, runGetCmd, runCreateCmd)
	pipelineCmd.AddCommand(runCmd)
}

// formatMilli 把毫秒时间戳格式化为本地时间，0 值返回 "-"。
func formatMilli(ms int64) string {
	if ms <= 0 {
		return "-"
	}
	return time.UnixMilli(ms).Format("2006-01-02 15:04:05")
}

// triggerModeName 把触发方式数字转为可读名称。
func triggerModeName(m int) string {
	switch m {
	case 1:
		return "人工"
	case 2:
		return "定时"
	case 3:
		return "代码提交"
	case 5:
		return "流水线"
	case 6:
		return "WEBHOOK"
	default:
		return fmt.Sprintf("未知(%d)", m)
	}
}

func init() {
	jobStartOpts.register(jobStartCmd)
	jobStartCmd.Flags().BoolVar(&jobStartOpts.outputJSON, "json", false, "以 JSON 输出结果")

	jobLogOpts.register(jobLogCmd)
	f := jobLogCmd.Flags()
	f.BoolVar(&jobLogOpts.follow, "follow", true, "持续获取日志直到任务结束（--follow=false 只取一次）")
	f.DurationVar(&jobLogOpts.interval, "interval", 2*time.Second, "持续获取时的轮询间隔")
	f.BoolVar(&jobLogOpts.outputJSON, "json", false, "以 JSON 输出单次查询结果（不持续获取）")

	pipelineJobCmd.AddCommand(jobStartCmd, jobLogCmd)
	pipelineCmd.AddCommand(pipelineJobCmd)
	rootCmd.AddCommand(pipelineCmd)
}
