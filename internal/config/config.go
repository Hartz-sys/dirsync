package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Name string `yaml:"name"`
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	User string `yaml:"user"`
	// AuthMethod 认证方式：password / key / auto（默认 auto）。
	AuthMethod           string   `yaml:"auth_method"`
	Password             string   `yaml:"password"`
	PrivateKey           string   `yaml:"private_key"`
	PrivateKeyPassphrase string   `yaml:"private_key_passphrase"`
	LocalDir             string   `yaml:"local_dir"`
	RemoteDir            string   `yaml:"remote_dir"`
	Exclude              []string `yaml:"exclude"`
	DeleteExtra          bool     `yaml:"delete_extra"`
	DryRun               bool     `yaml:"dry_run"`
	// BackupBeforeSync 默认开启；设为 false 可关闭同步前备份。
	BackupBeforeSync *bool `yaml:"backup_before_sync"`
	// BackupDir 备份根目录；为空时在 remote_dir 同级生成 remote_dir.bak.时间戳。
	BackupDir string `yaml:"backup_dir"`
}

const (
	AuthMethodAuto     = "auto"
	AuthMethodPassword = "password"
	AuthMethodKey      = "key"
)

// ResolvedAuthMethod 返回实际使用的认证方式。
func (c *Config) ResolvedAuthMethod() string {
	switch strings.ToLower(strings.TrimSpace(c.AuthMethod)) {
	case AuthMethodPassword, "pwd", "pass":
		return AuthMethodPassword
	case AuthMethodKey, "private_key", "pubkey", "ssh_key":
		return AuthMethodKey
	case "", AuthMethodAuto:
		if strings.TrimSpace(c.PrivateKey) != "" {
			return AuthMethodKey
		}
		return AuthMethodPassword
	default:
		return strings.ToLower(strings.TrimSpace(c.AuthMethod))
	}
}

// IsBackupEnabled 返回是否在同步前备份远端目录（默认 true）。
func (c *Config) IsBackupEnabled() bool {
	if c.BackupBeforeSync == nil {
		return true
	}
	return *c.BackupBeforeSync
}

func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(home, ".dirsync", "config.yaml")
}

func ResolveConfigPath(input string) (string, error) {
	if strings.TrimSpace(input) == "" {
		if _, err := os.Stat("config.yaml"); err == nil {
			return filepath.Abs("config.yaml")
		}
		return filepath.Abs(DefaultConfigPath())
	}
	return filepath.Abs(input)
}

func Load(path string) (*Config, error) {
	resolved, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("解析配置路径失败: %w", err)
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析 YAML 配置失败: %w", err)
	}

	if err := cfg.normalize(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Clone() *Config {
	dup := *c
	dup.Exclude = append([]string(nil), c.Exclude...)
	if c.BackupBeforeSync != nil {
		v := *c.BackupBeforeSync
		dup.BackupBeforeSync = &v
	}
	return &dup
}

func (c *Config) normalize() error {
	if c.Port == 0 {
		c.Port = 22
	}

	rawAuth := strings.ToLower(strings.TrimSpace(c.AuthMethod))
	switch rawAuth {
	case "", AuthMethodAuto:
		c.AuthMethod = AuthMethodAuto
	case AuthMethodPassword, "pwd", "pass":
		c.AuthMethod = AuthMethodPassword
	case AuthMethodKey, "private_key", "pubkey", "ssh_key":
		c.AuthMethod = AuthMethodKey
	default:
		return fmt.Errorf("auth_method 无效: %s（可选 password / key / auto）", c.AuthMethod)
	}

	var err error
	c.LocalDir, err = ExpandPath(c.LocalDir)
	if err != nil {
		return fmt.Errorf("解析 local_dir 失败: %w", err)
	}
	c.PrivateKey, err = ExpandPath(c.PrivateKey)
	if err != nil {
		return fmt.Errorf("解析 private_key 失败: %w", err)
	}

	c.RemoteDir = cleanRemotePath(c.RemoteDir)
	c.BackupDir = cleanRemotePath(c.BackupDir)
	return c.Validate()
}

func (c *Config) Validate() error {
	switch {
	case strings.TrimSpace(c.Host) == "":
		return errors.New("host 不能为空")
	case c.Port < 1 || c.Port > 65535:
		return errors.New("port 必须在 1-65535 之间")
	case strings.TrimSpace(c.User) == "":
		return errors.New("user 不能为空")
	case strings.TrimSpace(c.LocalDir) == "":
		return errors.New("local_dir 不能为空")
	case strings.TrimSpace(c.RemoteDir) == "":
		return errors.New("remote_dir 不能为空")
	}

	switch c.ResolvedAuthMethod() {
	case AuthMethodPassword:
		if strings.TrimSpace(c.Password) == "" {
			return errors.New("auth_method=password 时必须配置 password")
		}
	case AuthMethodKey:
		if strings.TrimSpace(c.PrivateKey) == "" {
			return errors.New("auth_method=key 时必须配置 private_key")
		}
	default:
		return fmt.Errorf("auth_method 无效: %s（可选 password / key / auto）", c.AuthMethod)
	}

	info, err := os.Stat(c.LocalDir)
	if err != nil {
		return fmt.Errorf("本地目录不可访问: %w", err)
	}
	if !info.IsDir() {
		return errors.New("local_dir 必须是目录")
	}
	return nil
}

func ExpandPath(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil
	}
	if input == "~" || strings.HasPrefix(input, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		input = filepath.Join(home, strings.TrimPrefix(input, "~/"))
	}
	return filepath.Abs(input)
}

func cleanRemotePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimRight(path, "/")
}
