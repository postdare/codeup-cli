package cli

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/postdare/codeup-cli/internal/api"
)

// fakeJobLogAPI 按调用次数依次返回预设的日志与任务状态。
type fakeJobLogAPI struct {
	logs      []*api.PipelineJobRunLog
	statuses  []*api.PipelineRunJob
	logCalls  int
	statCalls int
}

func (f *fakeJobLogAPI) GetPipelineJobRunLog(context.Context, string, string, string) (*api.PipelineJobRunLog, error) {
	i := f.logCalls
	if i >= len(f.logs) {
		i = len(f.logs) - 1
	}
	f.logCalls++
	return f.logs[i], nil
}

func (f *fakeJobLogAPI) GetPipelineJobStatus(context.Context, string, string, string) (*api.PipelineRunJob, error) {
	i := f.statCalls
	if i >= len(f.statuses) {
		i = len(f.statuses) - 1
	}
	f.statCalls++
	return f.statuses[i], nil
}

// captureStdout 在 fn 执行期间接管 os.Stdout 并返回输出内容。
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = orig })

	runErr := fn()

	_ = w.Close()
	os.Stdout = orig
	out, _ := io.ReadAll(r)
	return string(out), runErr
}

func testFlags() pipelineJobFlags {
	return pipelineJobFlags{pipelineID: "123", runID: "1", jobID: "21212"}
}

func TestFollowJobLogUntilSuccess(t *testing.T) {
	fake := &fakeJobLogAPI{
		logs: []*api.PipelineJobRunLog{
			{Content: "step1\n", More: true},
			{Content: "step1\nstep2\n", More: false},
		},
		statuses: []*api.PipelineRunJob{
			{Name: "部署", Status: "RUNNING"},
			{Name: "部署", Status: "SUCCESS"},
		},
	}

	out, err := captureStdout(t, func() error {
		return followJobLog(context.Background(), fake, testFlags(), time.Millisecond)
	})
	if err != nil {
		t.Fatal(err)
	}
	// 第二次返回的是全量内容，应按前缀去重，只增量输出 step2。
	if out != "step1\nstep2\n" {
		t.Errorf("stdout = %q, want %q", out, "step1\nstep2\n")
	}
}

func TestFollowJobLogFailReturnsError(t *testing.T) {
	fake := &fakeJobLogAPI{
		logs:     []*api.PipelineJobRunLog{{Content: "boom\n", More: false}},
		statuses: []*api.PipelineRunJob{{Name: "部署", Status: "FAIL"}},
	}

	_, err := captureStdout(t, func() error {
		return followJobLog(context.Background(), fake, testFlags(), time.Millisecond)
	})
	if err == nil || !strings.Contains(err.Error(), "FAIL") {
		t.Errorf("expected FAIL error, got %v", err)
	}
}

func TestFollowJobLogKeepsPollingWhileRunning(t *testing.T) {
	// 日志暂时取完（more=false）但任务仍在运行时，不应退出。
	fake := &fakeJobLogAPI{
		logs: []*api.PipelineJobRunLog{
			{Content: "a\n", More: false},
			{Content: "a\n", More: false},
			{Content: "a\nb\n", More: false},
		},
		statuses: []*api.PipelineRunJob{
			{Name: "j", Status: "RUNNING"},
			{Name: "j", Status: "RUNNING"},
			{Name: "j", Status: "SUCCESS"},
		},
	}

	out, err := captureStdout(t, func() error {
		return followJobLog(context.Background(), fake, testFlags(), time.Millisecond)
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "a\nb\n" {
		t.Errorf("stdout = %q, want %q", out, "a\nb\n")
	}
	if fake.logCalls < 3 {
		t.Errorf("logCalls = %d, want >= 3", fake.logCalls)
	}
}

func TestFollowJobLogContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fake := &fakeJobLogAPI{
		logs:     []*api.PipelineJobRunLog{{Content: "a\n", More: true}},
		statuses: []*api.PipelineRunJob{{Name: "j", Status: "RUNNING"}},
	}
	cancel()

	_, err := captureStdout(t, func() error {
		return followJobLog(ctx, fake, testFlags(), time.Millisecond)
	})
	if err != nil {
		t.Errorf("context cancel should exit cleanly, got %v", err)
	}
}
