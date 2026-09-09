package cli

import (
	"encoding/json"
	"testing"
)

func TestBuildRunParamsEmpty(t *testing.T) {
	got, err := buildRunParams(&runCreateParams{})
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("params = %q, want empty", got)
	}
}

func TestBuildRunParamsFull(t *testing.T) {
	got, err := buildRunParams(&runCreateParams{
		envs:       []string{"app=crm", "version=1.0"},
		branches:   []string{"https://example.com/repo.git=master"},
		branchMode: []string{"dev", "feat/x"},
		tags:       []string{"https://example.com/repo.git=v1.0"},
	})
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatalf("params 不是合法 JSON: %v", err)
	}
	if m["branchModeBranchs"] == nil || m["runningBranchs"] == nil || m["runningTags"] == nil || m["envs"] == nil {
		t.Errorf("缺少预期字段: %v", m)
	}
}

func TestBuildRunParamsInvalidKV(t *testing.T) {
	if _, err := buildRunParams(&runCreateParams{envs: []string{"noequals"}}); err == nil {
		t.Error("expected error for invalid key=value")
	}
	if _, err := buildRunParams(&runCreateParams{branches: []string{"noequals"}}); err == nil {
		t.Error("expected error for invalid url=branch")
	}
}

func TestBuildRunParamsInvalidExtraJSON(t *testing.T) {
	if _, err := buildRunParams(&runCreateParams{params: "{not-json"}); err == nil {
		t.Error("expected error for invalid --params JSON")
	}
}

func TestBuildRunParamsExtraJSONMerge(t *testing.T) {
	got, err := buildRunParams(&runCreateParams{
		envs:   []string{"a=1"},
		params: `{"comment":"release","envs":{"b":2}}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatal(err)
	}
	if m["comment"] != "release" {
		t.Errorf("comment = %v, want release", m["comment"])
	}
	// --params 中的同名 key 覆盖已有选项
	if _, ok := m["envs"].(map[string]any)["a"]; ok {
		t.Errorf("--params 的 envs 应覆盖 --env 选项: %v", m["envs"])
	}
}
