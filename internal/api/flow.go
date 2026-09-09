package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// PipelineJobRunLog 是任务运行日志的返回结果。
type PipelineJobRunLog struct {
	// Content 日志内容。
	Content string `json:"content"`
	// Last 已返回日志的最后偏移。
	Last int64 `json:"last"`
	// More 是否还有最新日志。
	More bool `json:"more"`
}

// ExecutePipelineJobRun 手动运行流水线任务。
// 接口返回 boolean，false 时视为启动失败。
func (c *Client) ExecutePipelineJobRun(ctx context.Context, pipelineID, pipelineRunID, jobID string) error {
	path := fmt.Sprintf("%s/pipelines/%s/pipelineRuns/%s/jobs/%s/start",
		c.flowBasePath(),
		url.PathEscape(pipelineID),
		url.PathEscape(pipelineRunID),
		url.PathEscape(jobID),
	)
	var ok bool
	if err := c.post(ctx, path, nil, &ok); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("启动流水线任务失败（接口返回 false），请确认任务处于可手动运行的状态")
	}
	return nil
}

// GetPipelineJobRunLog 查询任务运行日志。
func (c *Client) GetPipelineJobRunLog(ctx context.Context, pipelineID, pipelineRunID, jobID string) (*PipelineJobRunLog, error) {
	path := fmt.Sprintf("%s/pipelines/%s/runs/%s/job/%s/log",
		c.flowBasePath(),
		url.PathEscape(pipelineID),
		url.PathEscape(pipelineRunID),
		url.PathEscape(jobID),
	)
	var log PipelineJobRunLog
	if err := c.get(ctx, path, &log); err != nil {
		return nil, err
	}
	return &log, nil
}

// CreatePipelineRunOptions 是运行流水线的请求体。
type CreatePipelineRunOptions struct {
	// Params 运行参数 JSON 字符串（branchModeBranchs/envs/runningBranchs/runningTags 等），
	// 留空表示使用流水线默认配置运行。
	Params string `json:"params,omitempty"`
}

// CreatePipelineRun 运行流水线，返回新的运行实例 ID。
func (c *Client) CreatePipelineRun(ctx context.Context, pipelineID string, opts *CreatePipelineRunOptions) (int64, error) {
	path := fmt.Sprintf("%s/pipelines/%s/runs",
		c.flowBasePath(),
		url.PathEscape(pipelineID),
	)
	var runID int64
	if err := c.post(ctx, path, opts, &runID); err != nil {
		return 0, err
	}
	return runID, nil
}

// PipelineRunSummary 是流水线运行实例列表中的单条记录。
type PipelineRunSummary struct {
	CreatorAccountID string `json:"creatorAccountId"`
	PipelineID       int64  `json:"pipelineId"`
	PipelineRunID    int64  `json:"pipelineRunId"`
	StartTime        int64  `json:"startTime"`
	EndTime          int64  `json:"endTime"`
	TriggerMode      int    `json:"triggerMode"`
	// Status 运行状态（FAIL/SUCCESS/RUNNING），列表接口可能不返回。
	Status string `json:"status"`
}

// StatusOrUnknown 返回状态展示文本，空状态返回 "-"。
func (r *PipelineRunSummary) StatusOrUnknown() string {
	if r == nil || r.Status == "" {
		return "-"
	}
	return r.Status
}

// ListPipelineRunsOptions 是获取流水线运行实例列表的查询参数。
type ListPipelineRunsOptions struct {
	Page    int
	PerPage int
	// Status 过滤运行状态：FAIL / SUCCESS / RUNNING，留空返回全部。
	Status string
	// TriggerMode 过滤触发方式：1 人工 2 定时 3 代码提交 5 流水线 6 WEBHOOK，0 表示不过滤。
	TriggerMode int
}

// ListPipelineRuns 获取流水线运行实例列表（返回单页结果与响应头中的分页信息）。
func (c *Client) ListPipelineRuns(ctx context.Context, pipelineID string, opts ListPipelineRunsOptions) ([]PipelineRunSummary, http.Header, error) {
	page := opts.Page
	if page <= 0 {
		page = 1
	}
	perPage := opts.PerPage
	if perPage <= 0 {
		perPage = 10
	}

	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("perPage", strconv.Itoa(perPage))
	if opts.Status != "" {
		q.Set("status", opts.Status)
	}
	if opts.TriggerMode > 0 {
		q.Set("triggerMode", strconv.Itoa(opts.TriggerMode))
	}

	path := fmt.Sprintf("%s/pipelines/%s/runs?%s",
		c.flowBasePath(),
		url.PathEscape(pipelineID),
		q.Encode(),
	)

	var items []PipelineRunSummary
	headers, err := c.getWithHeaders(ctx, path, &items)
	if err != nil {
		return nil, nil, err
	}
	return items, headers, nil
}

// PipelineRunJob 是流水线运行实例中的单个任务。
type PipelineRunJob struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// PipelineRunStage 是流水线运行实例中的单个阶段。
type PipelineRunStage struct {
	Index     string `json:"index"`
	Name      string `json:"name"`
	StageInfo struct {
		Name   string           `json:"name"`
		Status string           `json:"status"`
		Jobs   []PipelineRunJob `json:"jobs"`
	} `json:"stageInfo"`
}

// PipelineRunSource 是运行实例的代码源信息。
type PipelineRunSource struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Sign string `json:"sign"`
	Data struct {
		Branch string `json:"branch"`
		Repo   string `json:"repo"`
	} `json:"data"`
}

// PipelineRunParam 是运行实例的全局参数（运行变量）。
type PipelineRunParam struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Encrypted bool   `json:"encrypted"`
}

// PipelineRun 是流水线运行实例（仅保留任务状态查询所需字段）。
type PipelineRun struct {
	PipelineID    int64               `json:"pipelineId"`
	PipelineRunID int64               `json:"pipelineRunId"`
	Status        string              `json:"status"`
	Stages        []PipelineRunStage  `json:"stages"`
	Sources       []PipelineRunSource `json:"sources"`
	GlobalParams  []PipelineRunParam  `json:"globalParams"`
}

// FindJob 按任务 ID 在运行实例中查找任务，找不到时返回 nil。
func (r *PipelineRun) FindJob(jobID int64) *PipelineRunJob {
	for i := range r.Stages {
		jobs := r.Stages[i].StageInfo.Jobs
		for j := range jobs {
			if jobs[j].ID == jobID {
				return &jobs[j]
			}
		}
	}
	return nil
}

// PipelineJobTerminal 判断任务状态是否为终态（不再产生新日志）。
// 已知的非终态为 未启动/排队/运行中，其余一律按终态处理，避免轮询不退出。
func PipelineJobTerminal(status string) bool {
	switch status {
	case "", "WAITING", "QUEUING", "RUNNING", "PAUSED":
		return false
	default:
		return true
	}
}

// GetPipelineRun 获取流水线运行实例。
func (c *Client) GetPipelineRun(ctx context.Context, pipelineID, pipelineRunID string) (*PipelineRun, error) {
	path := fmt.Sprintf("%s/pipelines/%s/runs/%s",
		c.flowBasePath(),
		url.PathEscape(pipelineID),
		url.PathEscape(pipelineRunID),
	)
	var run PipelineRun
	if err := c.get(ctx, path, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

// GetPipelineJobStatus 查询流水线运行实例中指定任务的状态。
// jobID 为字符串形式的任务 ID，内部会解析为数字与运行实例中的任务匹配。
func (c *Client) GetPipelineJobStatus(ctx context.Context, pipelineID, pipelineRunID, jobID string) (*PipelineRunJob, error) {
	id, err := strconv.ParseInt(jobID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("任务 ID %q 不是数字，无法查询任务状态", jobID)
	}
	run, err := c.GetPipelineRun(ctx, pipelineID, pipelineRunID)
	if err != nil {
		return nil, err
	}
	job := run.FindJob(id)
	if job == nil {
		return nil, fmt.Errorf("在流水线运行实例 %s 中找不到任务 %s", pipelineRunID, jobID)
	}
	return job, nil
}
