package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestExecutePipelineJobRunCentral(t *testing.T) {
	var gotMethod, gotPath, gotToken string

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		gotToken = r.Header.Get("x-yunxiao-token")
		_ = json.NewEncoder(w).Encode(true)
	})
	client.OrganizationID = "60d54f3daccf2bbd6659f3ad"

	if err := client.ExecutePipelineJobRun(context.Background(), "123", "1", "21212"); err != nil {
		t.Fatal(err)
	}

	wantPath := "/oapi/v1/flow/organizations/60d54f3daccf2bbd6659f3ad/pipelines/123/pipelineRuns/1/jobs/21212/start"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotToken != "pt-test-token" {
		t.Errorf("token header = %q", gotToken)
	}
}

func TestExecutePipelineJobRunRegion(t *testing.T) {
	var gotPath string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		_ = json.NewEncoder(w).Encode(true)
	})

	if err := client.ExecutePipelineJobRun(context.Background(), "123", "1", "21212"); err != nil {
		t.Fatal(err)
	}

	wantPath := "/oapi/v1/flow/pipelines/123/pipelineRuns/1/jobs/21212/start"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}
}

func TestExecutePipelineJobRunReturnsFalse(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(false)
	})

	err := client.ExecutePipelineJobRun(context.Background(), "123", "1", "21212")
	if err == nil {
		t.Fatal("expected error when API returns false")
	}
}

func TestGetPipelineJobRunLog(t *testing.T) {
	var gotMethod, gotPath string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": "building...\nsuccess\n",
			"last":    2,
			"more":    false,
		})
	})
	client.OrganizationID = "org-1"

	log, err := client.GetPipelineJobRunLog(context.Background(), "123", "1", "21212")
	if err != nil {
		t.Fatal(err)
	}

	wantPath := "/oapi/v1/flow/organizations/org-1/pipelines/123/runs/1/job/21212/log"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if log.Content != "building...\nsuccess\n" || log.Last != 2 || log.More {
		t.Errorf("unexpected log: %+v", log)
	}
}

func TestGetPipelineJobStatus(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"pipelineId":    123,
			"pipelineRunId": 1,
			"status":        "RUNNING",
			"stages": []map[string]any{
				{
					"index": "Group0-Stage0",
					"name":  "Java 构建",
					"stageInfo": map[string]any{
						"name":   "Java 构建",
						"status": "RUNNING",
						"jobs": []map[string]any{
							{"id": 111, "name": "构建", "status": "SUCCESS"},
							{"id": 21212, "name": "部署", "status": "RUNNING"},
						},
					},
				},
			},
		})
	})

	job, err := client.GetPipelineJobStatus(context.Background(), "123", "1", "21212")
	if err != nil {
		t.Fatal(err)
	}
	if job.Name != "部署" || job.Status != "RUNNING" {
		t.Errorf("unexpected job: %+v", job)
	}

	if _, err := client.GetPipelineJobStatus(context.Background(), "123", "1", "999"); err == nil {
		t.Error("expected error for unknown job id")
	}
	if _, err := client.GetPipelineJobStatus(context.Background(), "123", "1", "abc"); err == nil {
		t.Error("expected error for non-numeric job id")
	}
}

func TestCreatePipelineRun(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`42`))
	})
	client.OrganizationID = "org-1"

	runID, err := client.CreatePipelineRun(context.Background(), "123", &CreatePipelineRunOptions{
		Params: `{"envs":{"k1":"v1"}}`,
	})
	if err != nil {
		t.Fatal(err)
	}

	wantPath := "/oapi/v1/flow/organizations/org-1/pipelines/123/runs"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if runID != 42 {
		t.Errorf("runID = %d, want 42", runID)
	}
	if gotBody["params"] != `{"envs":{"k1":"v1"}}` {
		t.Errorf("unexpected body: %v", gotBody)
	}
}

func TestCreatePipelineRunNoParams(t *testing.T) {
	var gotBody map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`7`))
	})

	runID, err := client.CreatePipelineRun(context.Background(), "123", &CreatePipelineRunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if runID != 7 {
		t.Errorf("runID = %d, want 7", runID)
	}
	if _, ok := gotBody["params"]; ok {
		t.Error("空 params 不应出现在请求体中")
	}
}

func TestPipelineJobTerminal(t *testing.T) {
	nonTerminal := []string{"", "WAITING", "QUEUING", "RUNNING", "PAUSED"}
	for _, s := range nonTerminal {
		if PipelineJobTerminal(s) {
			t.Errorf("PipelineJobTerminal(%q) = true, want false", s)
		}
	}
	terminal := []string{"SUCCESS", "FAIL", "CANCELED", "SKIPPED", "SOME_UNKNOWN_STATUS"}
	for _, s := range terminal {
		if !PipelineJobTerminal(s) {
			t.Errorf("PipelineJobTerminal(%q) = false, want true", s)
		}
	}
}
