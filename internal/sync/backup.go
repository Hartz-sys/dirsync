package sync

import (
	"bytes"
	"fmt"
	"path"
	"strings"
	"time"

	appconfig "dirsync/internal/config"
)

// backupRemote 在同步前将远端目录复制到备份路径。
// 远端目录不存在时跳过备份；通过 SSH 执行 cp -a，避免 SFTP 逐文件复制。
func (s *Service) backupRemote(remoteDir string, cfg *appconfig.Config) (string, error) {
	info, err := s.client.Stat(remoteDir)
	if err != nil {
		if isSFTPNotExist(err) {
			fmt.Printf("远端目录不存在，跳过备份: %s\n", remoteDir)
			return "", nil
		}
		return "", fmt.Errorf("检查远端目录失败: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("远端路径不是目录，无法备份: %s", remoteDir)
	}

	backupPath := buildBackupPath(remoteDir, cfg.BackupDir, time.Now())
	if cfg.DryRun {
		fmt.Printf("[dry-run] 备份远端目录 %s -> %s\n", remoteDir, backupPath)
		return backupPath, nil
	}

	parent := path.Dir(backupPath)
	script := fmt.Sprintf(
		"mkdir -p %s && cp -a %s %s",
		shellQuote(parent),
		shellQuote(remoteDir),
		shellQuote(backupPath),
	)

	session, err := s.ssh.NewSession()
	if err != nil {
		return "", fmt.Errorf("创建 SSH 会话失败: %w", err)
	}
	defer session.Close()

	var stderr bytes.Buffer
	session.Stderr = &stderr
	if err := session.Run(script); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("备份远端目录失败: %s", msg)
	}

	fmt.Printf("已备份远端目录: %s -> %s\n", remoteDir, backupPath)
	return backupPath, nil
}

func buildBackupPath(remoteDir, backupDir string, now time.Time) string {
	stamp := now.Format("20060102150405")
	base := path.Base(remoteDir)
	if strings.TrimSpace(backupDir) == "" {
		return fmt.Sprintf("%s.bak.%s", remoteDir, stamp)
	}
	return path.Join(backupDir, fmt.Sprintf("%s-%s", base, stamp))
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
