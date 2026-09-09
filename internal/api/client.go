// Package api 封装阿里云云效 Codeup OpenAPI 的 HTTP 客户端。
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client 是云效 OpenAPI 客户端。
type Client struct {
	// Domain 服务接入点，如 openapi-rdc.aliyuncs.com。
	Domain string
	// Token 个人访问令牌。
	Token string
	// OrganizationID 组织 ID。非空时按中心版路径调用，否则按 Region 版。
	OrganizationID string

	httpClient *http.Client
}

// NewClient 创建客户端。
func NewClient(domain, token, organizationID string) *Client {
	return &Client{
		Domain:         strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(domain, "https://"), "http://"), "/"),
		Token:          token,
		OrganizationID: organizationID,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
	}
}

// APIError 表示服务端返回的非 2xx 响应。
type APIError struct {
	StatusCode int
	Code       string `json:"errorCode"`
	Message    string `json:"errorMessage"`
	RequestID  string `json:"requestId"`
	Raw        string
}

func (e *APIError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "API 请求失败 (HTTP %d)", e.StatusCode)
	if e.Code != "" {
		fmt.Fprintf(&b, " [%s]", e.Code)
	}
	if e.Message != "" {
		fmt.Fprintf(&b, ": %s", e.Message)
	} else if e.Raw != "" {
		fmt.Fprintf(&b, ": %s", e.Raw)
	}
	if e.RequestID != "" {
		fmt.Fprintf(&b, " (requestId: %s)", e.RequestID)
	}
	return b.String()
}

// codeupBasePath 返回 codeup 资源的基础路径，按是否配置组织 ID 区分中心版/Region 版。
func (c *Client) codeupBasePath() string {
	if c.OrganizationID != "" {
		return fmt.Sprintf("/oapi/v1/codeup/organizations/%s", url.PathEscape(c.OrganizationID))
	}
	return "/oapi/v1/codeup"
}

// flowBasePath 返回流水线（flow）资源的基础路径，按是否配置组织 ID 区分中心版/Region 版。
func (c *Client) flowBasePath() string {
	if c.OrganizationID != "" {
		return fmt.Sprintf("/oapi/v1/flow/organizations/%s", url.PathEscape(c.OrganizationID))
	}
	return "/oapi/v1/flow"
}

// do 是通用 HTTP 请求方法。body 为 nil 时不发送请求体（用于 GET）。
func (c *Client) do(ctx context.Context, method, path string, body, result any) error {
	// 注意：必须声明为 io.Reader，若声明为 *bytes.Reader，nil 指针装入接口后
	// http.NewRequestWithContext 会对 nil *bytes.Reader 调用 Len() 导致空指针 panic。
	var reqBody io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("序列化请求体失败: %w", err)
		}
		reqBody = bytes.NewReader(payload)
	}

	u := "https://" + c.Domain + path
	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("x-yunxiao-token", c.Token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求 %s 失败: %w", u, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &APIError{StatusCode: resp.StatusCode, Raw: strings.TrimSpace(string(respBody))}
		_ = json.Unmarshal(respBody, apiErr)
		return apiErr
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("解析响应失败: %w\n原始响应: %s", err, respBody)
		}
	}
	return nil
}

// doWithHeaders 是通用 HTTP 请求方法，额外返回响应头（分页信息在 x-total 等自定义头中）。
func (c *Client) doWithHeaders(ctx context.Context, method, path string, body, result any) (http.Header, error) {
	u := "https://" + c.Domain + path
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return nil, err
	}
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("序列化请求体失败: %w", err)
		}
		req.Body = io.NopCloser(bytes.NewReader(payload))
		req.ContentLength = int64(len(payload))
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("x-yunxiao-token", c.Token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 %s 失败: %w", u, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &APIError{StatusCode: resp.StatusCode, Raw: strings.TrimSpace(string(respBody))}
		_ = json.Unmarshal(respBody, apiErr)
		return nil, apiErr
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return nil, fmt.Errorf("解析响应失败: %w\n原始响应: %s", err, respBody)
		}
	}
	return resp.Header, nil
}

// get 发起 GET 请求。
func (c *Client) get(ctx context.Context, path string, result any) error {
	_, err := c.getWithHeaders(ctx, path, result)
	return err
}

// getWithHeaders 发起 GET 请求并返回响应头。
func (c *Client) getWithHeaders(ctx context.Context, path string, result any) (http.Header, error) {
	return c.doWithHeaders(ctx, http.MethodGet, path, nil, result)
}

// post 发起 POST 请求。
func (c *Client) post(ctx context.Context, path string, body, result any) error {
	return c.do(ctx, http.MethodPost, path, body, result)
}
