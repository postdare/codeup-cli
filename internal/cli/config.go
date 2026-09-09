package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/postdare/codeup-cli/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "管理 CLI 配置",
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "写入配置项到配置文件",
	Long:  "支持的配置项: domain（服务接入点）、token（个人访问令牌）、org-id（组织 ID，仅中心版需要）。",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value := args[0], args[1]

		// 只读文件、不叠加环境变量，避免把环境变量值固化进配置文件。
		cfg, err := config.LoadFile()
		if err != nil {
			return err
		}

		switch key {
		case "domain":
			cfg.Domain = value
		case "token":
			cfg.Token = value
		case "org-id", "organization-id":
			cfg.OrganizationID = value
		default:
			return fmt.Errorf("未知配置项 %q，支持: domain, token, org-id", key)
		}
		if err := cfg.Save(); err != nil {
			return err
		}
		path, _ := config.Path()
		fmt.Printf("已写入 %s\n", path)
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get",
	Short: "查看当前生效的配置（含环境变量与参数覆盖）",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		fmt.Printf("domain: %s\n", valueOrUnset(cfg.Domain))
		fmt.Printf("token:  %s\n", valueOrUnset(maskToken(cfg.Token)))
		fmt.Printf("org-id: %s （留空时按 Region 版调用）\n", valueOrUnset(cfg.OrganizationID))
		return nil
	},
}

func valueOrUnset(v string) string {
	if v == "" {
		return "(未设置)"
	}
	return v
}

// maskToken 仅保留令牌首尾各 4 个字符。
func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return strings.Repeat("*", len(token))
	}
	return token[:4] + strings.Repeat("*", 4) + token[len(token)-4:]
}

func init() {
	configCmd.AddCommand(configSetCmd, configGetCmd)
	rootCmd.AddCommand(configCmd)
}
