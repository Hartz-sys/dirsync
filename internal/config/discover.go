package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Candidate 表示可交互选择的配置文件。
type Candidate struct {
	Path       string
	Name       string
	Host       string
	Port       int
	User       string
	AuthMethod string
	RemoteDir  string
	LocalDir   string
}

// Peek 读取配置摘要，不做完整校验（用于菜单展示）。
func Peek(path string) (Candidate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Candidate{}, err
	}

	var raw struct {
		Name       string `yaml:"name"`
		Host       string `yaml:"host"`
		Port       int    `yaml:"port"`
		User       string `yaml:"user"`
		AuthMethod string `yaml:"auth_method"`
		Password   string `yaml:"password"`
		PrivateKey string `yaml:"private_key"`
		LocalDir   string `yaml:"local_dir"`
		RemoteDir  string `yaml:"remote_dir"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Candidate{}, err
	}

	port := raw.Port
	if port == 0 {
		port = 22
	}
	name := strings.TrimSpace(raw.Name)
	if name == "" {
		name = displayNameFromPath(path)
	}

	tmp := &Config{
		AuthMethod: raw.AuthMethod,
		Password:   raw.Password,
		PrivateKey: raw.PrivateKey,
	}
	auth := strings.ToLower(strings.TrimSpace(raw.AuthMethod))
	switch auth {
	case AuthMethodPassword, "pwd", "pass":
		auth = AuthMethodPassword
	case AuthMethodKey, "private_key", "pubkey", "ssh_key":
		auth = AuthMethodKey
	case "", AuthMethodAuto:
		auth = tmp.ResolvedAuthMethod()
	}

	return Candidate{
		Path:       path,
		Name:       name,
		Host:       strings.TrimSpace(raw.Host),
		Port:       port,
		User:       strings.TrimSpace(raw.User),
		AuthMethod: auth,
		RemoteDir:  cleanRemotePath(raw.RemoteDir),
		LocalDir:   strings.TrimSpace(raw.LocalDir),
	}, nil
}

func (c Candidate) Summary() string {
	host := c.Host
	if host == "" {
		host = "?"
	}
	if c.Port != 0 && c.Port != 22 {
		host = fmt.Sprintf("%s:%d", host, c.Port)
	}
	remote := c.RemoteDir
	if remote == "" {
		remote = "?"
	}
	userPart := c.User
	if userPart == "" {
		userPart = "?"
	}
	auth := c.AuthMethod
	if auth == "" {
		auth = AuthMethodAuto
	}
	return fmt.Sprintf("%s  %s@%s:%s  [%s]", c.Name, userPart, host, remote, auth)
}

// ListCandidates 扫描当前目录和 ~/.dirsync 下的可用配置。
func ListCandidates() ([]Candidate, error) {
	seen := make(map[string]struct{})
	var paths []string

	addGlob := func(pattern string) {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return
		}
		for _, match := range matches {
			base := filepath.Base(match)
			if base == "config.example.yaml" {
				continue
			}
			if !strings.HasSuffix(base, ".yaml") && !strings.HasSuffix(base, ".yml") {
				continue
			}
			abs, err := filepath.Abs(match)
			if err != nil {
				continue
			}
			if _, ok := seen[abs]; ok {
				continue
			}
			seen[abs] = struct{}{}
			paths = append(paths, abs)
		}
	}

	addGlob("config.yaml")
	addGlob("config.yml")
	addGlob("config.*.yaml")
	addGlob("config.*.yml")
	addGlob("*.sync.yaml")
	addGlob("*.sync.yml")

	if home, err := os.UserHomeDir(); err == nil {
		dir := filepath.Join(home, ".dirsync")
		addGlob(filepath.Join(dir, "config.yaml"))
		addGlob(filepath.Join(dir, "config.yml"))
		addGlob(filepath.Join(dir, "config.*.yaml"))
		addGlob(filepath.Join(dir, "config.*.yml"))
		addGlob(filepath.Join(dir, "*.sync.yaml"))
		addGlob(filepath.Join(dir, "*.sync.yml"))
	}

	sort.Strings(paths)

	var candidates []Candidate
	for _, path := range paths {
		item, err := Peek(path)
		if err != nil {
			continue
		}
		candidates = append(candidates, item)
	}
	return candidates, nil
}

func displayNameFromPath(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	switch {
	case base == "config":
		return "default"
	case strings.HasPrefix(base, "config."):
		return strings.TrimPrefix(base, "config.")
	case strings.HasSuffix(base, ".sync"):
		return strings.TrimSuffix(base, ".sync")
	default:
		return base
	}
}
