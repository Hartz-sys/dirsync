package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	appconfig "dirsync/internal/config"
	"dirsync/internal/sshauth"
	syncsvc "dirsync/internal/sync"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const exampleConfig = `name: default
host: 192.168.1.100
port: 22
user: deploy

# 认证方式：password / key / auto（默认 auto，有 private_key 时用密钥）
auth_method: auto
password: "your-password"
private_key: "~/.ssh/id_rsa"
# private_key_passphrase: ""

local_dir: "/Users/you/project"
remote_dir: "/opt/app"

exclude:
  - ".git"
  - "node_modules"
  - "*.log"

delete_extra: false
dry_run: false

# 同步前备份远端目录（默认开启）；backup_dir 为空时生成 remote_dir.bak.时间戳
backup_before_sync: true
# backup_dir: "/opt/backups"
`

func main() {
	if len(os.Args) < 2 {
		if err := runInteractive(); err != nil {
			exitWithError(err)
		}
		return
	}

	switch os.Args[1] {
	case "init":
		if err := runInit(os.Args[2:]); err != nil {
			exitWithError(err)
		}
	case "check":
		if err := runCheck(os.Args[2:]); err != nil {
			exitWithError(err)
		}
	case "sync":
		if err := runSync(os.Args[2:]); err != nil {
			exitWithError(err)
		}
	case "-h", "--help", "help":
		printUsage()
	default:
		printUsage()
		exitWithError(fmt.Errorf("未知命令: %s", os.Args[1]))
	}
}

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	target := fs.String("c", "config.yaml", "输出配置文件路径")
	if err := fs.Parse(args); err != nil {
		return err
	}

	absTarget, err := filepath.Abs(*target)
	if err != nil {
		return fmt.Errorf("解析输出路径失败: %w", err)
	}
	if _, err := os.Stat(absTarget); err == nil {
		return fmt.Errorf("配置文件已存在: %s", absTarget)
	}

	if err := os.MkdirAll(filepath.Dir(absTarget), 0o755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	if err := os.WriteFile(absTarget, []byte(exampleConfig), 0o600); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	fmt.Printf("已生成配置文件: %s\n", absTarget)
	return nil
}

func runCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	configPath := fs.String("c", "", "配置文件路径")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, resolvedPath, err := loadConfig(*configPath)
	if err != nil {
		return err
	}

	sshClient, sftpClient, cleanup, err := connect(cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	svc := syncsvc.NewService(sshClient, sftpClient, cfg)
	if err := svc.CheckConnection(cfg.RemoteDir); err != nil {
		return err
	}

	fmt.Printf("配置检查通过: %s\n", resolvedPath)
	fmt.Printf("已成功连接到 %s:%d，远端目录可访问或可创建。\n", cfg.Host, cfg.Port)
	return nil
}

func runSync(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	configPath := fs.String("c", "", "配置文件路径")
	localOverride := fs.String("local", "", "临时覆盖本地目录")
	remoteOverride := fs.String("remote", "", "临时覆盖远端目录")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, resolvedPath, err := loadConfig(*configPath)
	if err != nil {
		return err
	}

	sshClient, sftpClient, cleanup, err := connect(cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	svc := syncsvc.NewService(sshClient, sftpClient, cfg)
	summary, err := svc.Run(syncsvc.Options{
		LocalOverride:  *localOverride,
		RemoteOverride: *remoteOverride,
	})
	if err != nil {
		return err
	}

	fmt.Printf("\n同步完成: %s\n", resolvedPath)
	if summary.BackupPath != "" {
		fmt.Printf("远端备份: %s\n", summary.BackupPath)
	}
	fmt.Printf("上传 %d 个文件，跳过 %d 个文件，删除 %d 个文件，失败 %d 个文件，传输 %d 字节。\n",
		summary.UploadedFiles, summary.SkippedFiles, summary.DeletedFiles, summary.FailedFiles, summary.UploadedBytes)
	return nil
}

func loadConfig(configPath string) (*appconfig.Config, string, error) {
	resolvedPath, err := appconfig.ResolveConfigPath(configPath)
	if err != nil {
		return nil, "", err
	}
	cfg, err := appconfig.Load(resolvedPath)
	if err != nil {
		return nil, "", err
	}
	return cfg, resolvedPath, nil
}

func connect(cfg *appconfig.Config) (*ssh.Client, *sftp.Client, func(), error) {
	sshClient, err := sshauth.Dial(cfg)
	if err != nil {
		return nil, nil, nil, err
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, nil, nil, fmt.Errorf("建立 SFTP 会话失败: %w", err)
	}

	cleanup := func() {
		_ = sftpClient.Close()
		_ = sshClient.Close()
	}
	return sshClient, sftpClient, cleanup, nil
}

func printUsage() {
	fmt.Print(`dirsync - Mac 到 Linux 目录同步工具

用法:
  dirsync                              # 交互选择配置与操作
  dirsync init [-c config.yaml]
  dirsync check [-c config.yaml]
  dirsync sync [-c config.yaml] [--local /path] [--remote /path]

多目标配置示例:
  config.yaml / config.dev.yaml / config.prod.yaml
  或 *.sync.yaml，可在配置中设置 name 作为菜单显示名
`)
}

func exitWithError(err error) {
	fmt.Fprintf(os.Stderr, "错误: %v\n", err)
	os.Exit(1)
}
