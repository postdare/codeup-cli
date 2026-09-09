// Package cli 定义 codeup CLI 的命令树。
package cli

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"

	"github.com/postdare/codeup-cli/internal/api"
	"github.com/postdare/codeup-cli/internal/config"
)

// version 由构建时 ldflags 注入：-X github.com/postdare/codeup-cli/internal/cli.version=x.y.z
var version = "dev"

// 全局参数，可覆盖配置文件与环境变量。
var (
	flagDomain string
	flagToken  string
	flagOrgID  string
)

var rootCmd = &cobra.Command{
	Use:   "codeup",
	Short: "阿里云云效 Codeup 命令行工具",
	// Long 在 init 中根据当前配置状态动态生成。
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	// ldflags 未注入版本时（如 go install），回退到模块构建信息里的版本号。
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
	}
	rootCmd.Version = version

	rootCmd.PersistentFlags().StringVar(&flagDomain, "domain", "", "云效服务接入点（如 openapi-rdc.aliyuncs.com）")
	rootCmd.PersistentFlags().StringVar(&flagToken, "token", "", "个人访问令牌")
	rootCmd.PersistentFlags().StringVar(&flagOrgID, "org-id", "", "组织 ID（仅中心版需要，Region 版留空）")

	rootCmd.Long = buildLong()
}

// buildLong 生成根命令的帮助文本。已完成基础配置（domain + token）时，
// 不再展示初始化引导，只保留使用示例。
func buildLong() string {
	header := `codeup 是阿里云云效（Codeup）的命令行工具。

配置优先级：命令行参数 > 环境变量 (CODEUP_TOKEN / CODEUP_DOMAIN / CODEUP_ORG_ID) > 配置文件 (~/.config/codeup/config.json)。`

	example := `

示例:
  codeup mr create --repo 2813489 --source feat/demo --target master --title "my mr"`

	if cfg, err := config.Load(); err == nil && cfg.Domain != "" && cfg.Token != "" {
		return header + example
	}

	return header + `

快速开始:
  codeup config set domain openapi-rdc.aliyuncs.com
  codeup config set token pt-xxxx
  codeup config set org-id 60d54f3daccf2bbd6659f3ad   # 仅中心版需要` + example
}

// Execute 是 CLI 入口。
func Execute() error {
	return rootCmd.Execute()
}

// loadConfig 加载配置并应用命令行参数覆盖。
func loadConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if flagDomain != "" {
		cfg.Domain = flagDomain
	}
	if flagToken != "" {
		cfg.Token = flagToken
	}
	if flagOrgID != "" {
		cfg.OrganizationID = flagOrgID
	}
	return cfg, nil
}

// newAPIClient 加载配置并校验必填项，返回 API 客户端。
func newAPIClient() (*api.Client, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	if cfg.Domain == "" {
		return nil, fmt.Errorf("未配置服务接入点，请使用 --domain、环境变量 %s 或 `codeup config set domain <domain>`", config.EnvDomain)
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("未配置个人访问令牌，请使用 --token、环境变量 %s 或 `codeup config set token <token>`", config.EnvToken)
	}
	return api.NewClient(cfg.Domain, cfg.Token, cfg.OrganizationID), nil
}
