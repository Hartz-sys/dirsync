package sync

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	appconfig "dirsync/internal/config"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type Options struct {
	LocalOverride  string
	RemoteOverride string
}

type Summary struct {
	UploadedFiles int
	SkippedFiles  int
	DeletedFiles  int
	FailedFiles   int
	UploadedBytes int64
	BackupPath    string
}

type Service struct {
	ssh    *ssh.Client
	client *sftp.Client
	cfg    *appconfig.Config
}

func NewService(sshClient *ssh.Client, client *sftp.Client, cfg *appconfig.Config) *Service {
	return &Service{ssh: sshClient, client: client, cfg: cfg}
}

func (s *Service) Run(opts Options) (Summary, error) {
	cfg := s.cfg.Clone()
	if strings.TrimSpace(opts.LocalOverride) != "" {
		local, err := appconfig.ExpandPath(opts.LocalOverride)
		if err != nil {
			return Summary{}, fmt.Errorf("解析 --local 失败: %w", err)
		}
		cfg.LocalDir = local
	}
	if strings.TrimSpace(opts.RemoteOverride) != "" {
		cfg.RemoteDir = cleanRemotePath(opts.RemoteOverride)
	}
	if err := cfg.Validate(); err != nil {
		return Summary{}, err
	}

	summary := Summary{}
	localFiles := make(map[string]struct{})

	if cfg.IsBackupEnabled() {
		backupPath, err := s.backupRemote(cfg.RemoteDir, cfg)
		if err != nil {
			return summary, err
		}
		summary.BackupPath = backupPath
	}

	if err := s.client.MkdirAll(cfg.RemoteDir); err != nil && !isSFTPNotExist(err) {
		return summary, fmt.Errorf("创建远端根目录失败: %w", err)
	}

	err := filepath.WalkDir(cfg.LocalDir, func(filePath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if filePath == cfg.LocalDir {
			return nil
		}

		relPath, err := filepath.Rel(cfg.LocalDir, filePath)
		if err != nil {
			return err
		}

		relPath = filepath.ToSlash(relPath)
		if shouldExclude(relPath, d.IsDir(), cfg.Exclude) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		remotePath := path.Join(cfg.RemoteDir, relPath)
		localFiles[relPath] = struct{}{}

		if d.IsDir() {
			return s.ensureRemoteDir(remotePath, cfg.DryRun)
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		return s.syncFile(filePath, remotePath, relPath, info, cfg, &summary)
	})
	if err != nil {
		return summary, fmt.Errorf("遍历本地目录失败: %w", err)
	}

	if cfg.DeleteExtra {
		if err := s.deleteExtra(cfg.RemoteDir, localFiles, cfg, &summary); err != nil {
			return summary, err
		}
	}

	return summary, nil
}

func (s *Service) CheckConnection(remoteDir string) error {
	target := cleanRemotePath(remoteDir)
	if target == "" {
		target = s.cfg.RemoteDir
	}
	if _, err := s.client.Stat(target); err != nil {
		if isSFTPNotExist(err) {
			return nil
		}
		return fmt.Errorf("检查远端目录失败: %w", err)
	}
	return nil
}

func (s *Service) ensureRemoteDir(remotePath string, dryRun bool) error {
	if dryRun {
		fmt.Printf("[dry-run] 创建目录 %s\n", remotePath)
		return nil
	}
	if err := s.client.MkdirAll(remotePath); err != nil {
		return fmt.Errorf("创建远端目录失败 %s: %w", remotePath, err)
	}
	return nil
}

func (s *Service) syncFile(localPath, remotePath, relPath string, info fs.FileInfo, cfg *appconfig.Config, summary *Summary) error {
	needUpload, err := s.needsUpload(remotePath, info)
	if err != nil {
		summary.FailedFiles++
		return err
	}
	if !needUpload {
		summary.SkippedFiles++
		fmt.Printf("跳过 %s\n", relPath)
		return nil
	}

	if cfg.DryRun {
		summary.UploadedFiles++
		summary.UploadedBytes += info.Size()
		fmt.Printf("[dry-run] 上传 %s -> %s\n", relPath, remotePath)
		return nil
	}

	if err := s.client.MkdirAll(path.Dir(remotePath)); err != nil {
		summary.FailedFiles++
		return fmt.Errorf("创建远端父目录失败 %s: %w", remotePath, err)
	}

	if err := uploadFile(s.client, localPath, remotePath, info); err != nil {
		summary.FailedFiles++
		return fmt.Errorf("上传文件失败 %s: %w", relPath, err)
	}

	summary.UploadedFiles++
	summary.UploadedBytes += info.Size()
	fmt.Printf("上传 %s -> %s\n", relPath, remotePath)
	return nil
}

func (s *Service) needsUpload(remotePath string, localInfo fs.FileInfo) (bool, error) {
	remoteInfo, err := s.client.Stat(remotePath)
	if err != nil {
		if isSFTPNotExist(err) {
			return true, nil
		}
		return false, fmt.Errorf("读取远端文件信息失败 %s: %w", remotePath, err)
	}

	if remoteInfo.Size() != localInfo.Size() {
		return true, nil
	}

	delta := remoteInfo.ModTime().Sub(localInfo.ModTime())
	if delta < 0 {
		delta = -delta
	}
	return delta > time.Second, nil
}

func (s *Service) deleteExtra(remoteRoot string, localFiles map[string]struct{}, cfg *appconfig.Config, summary *Summary) error {
	remoteFiles, err := collectRemoteFiles(s.client, remoteRoot)
	if err != nil {
		return fmt.Errorf("收集远端文件失败: %w", err)
	}

	sort.Sort(sort.Reverse(sort.StringSlice(remoteFiles)))
	for _, relPath := range remoteFiles {
		if shouldExclude(relPath, false, cfg.Exclude) {
			continue
		}
		if _, ok := localFiles[relPath]; ok {
			continue
		}

		target := path.Join(remoteRoot, relPath)
		if cfg.DryRun {
			fmt.Printf("[dry-run] 删除远端多余文件 %s\n", target)
			continue
		}
		if err := s.client.Remove(target); err != nil && !isSFTPNotExist(err) {
			return fmt.Errorf("删除远端文件失败 %s: %w", target, err)
		}
		summary.DeletedFiles++
		fmt.Printf("删除远端多余文件 %s\n", target)
	}

	return cleanupRemoteDirs(s.client, remoteRoot, cfg)
}

func collectRemoteFiles(client *sftp.Client, remoteRoot string) ([]string, error) {
	var files []string
	walker := client.Walk(remoteRoot)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return nil, err
		}
		if walker.Path() == remoteRoot || walker.Stat().IsDir() {
			continue
		}
		relPath := strings.TrimPrefix(walker.Path(), strings.TrimRight(remoteRoot, "/")+"/")
		files = append(files, relPath)
	}
	return files, nil
}

func cleanupRemoteDirs(client *sftp.Client, remoteRoot string, cfg *appconfig.Config) error {
	var dirs []string
	walker := client.Walk(remoteRoot)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return err
		}
		if !walker.Stat().IsDir() || walker.Path() == remoteRoot {
			continue
		}
		relPath := strings.TrimPrefix(walker.Path(), strings.TrimRight(remoteRoot, "/")+"/")
		if shouldExclude(relPath, true, cfg.Exclude) {
			continue
		}
		dirs = append(dirs, walker.Path())
	}

	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	for _, dir := range dirs {
		entries, err := client.ReadDir(dir)
		if err != nil {
			if isSFTPNotExist(err) {
				continue
			}
			return err
		}
		if len(entries) > 0 {
			continue
		}
		if cfg.DryRun {
			fmt.Printf("[dry-run] 删除空目录 %s\n", dir)
			continue
		}
		if err := client.RemoveDirectory(dir); err != nil && !isSFTPNotExist(err) {
			return err
		}
		fmt.Printf("删除空目录 %s\n", dir)
	}
	return nil
}

func uploadFile(client *sftp.Client, localPath, remotePath string, info fs.FileInfo) error {
	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()

	tmpRemote := remotePath + ".dirsync.tmp"
	dst, err := client.OpenFile(tmpRemote, os.O_CREATE|os.O_TRUNC|os.O_WRONLY)
	if err != nil {
		return err
	}

	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		_ = client.Remove(tmpRemote)
		return err
	}
	if err := dst.Close(); err != nil {
		_ = client.Remove(tmpRemote)
		return err
	}

	if err := client.Chtimes(tmpRemote, info.ModTime(), info.ModTime()); err != nil {
		_ = client.Remove(tmpRemote)
		return err
	}

	if err := client.Rename(tmpRemote, remotePath); err != nil {
		if removeErr := client.Remove(remotePath); removeErr != nil && !isSFTPNotExist(removeErr) {
			_ = client.Remove(tmpRemote)
			return fmt.Errorf("替换远端文件失败: %w", removeErr)
		}
		if retryErr := client.Rename(tmpRemote, remotePath); retryErr != nil {
			_ = client.Remove(tmpRemote)
			return retryErr
		}
		return nil
	}

	return nil
}

func shouldExclude(relPath string, isDir bool, patterns []string) bool {
	base := path.Base(relPath)
	dirPrefix := relPath + "/"
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(filepath.ToSlash(pattern))
		if pattern == "" {
			continue
		}
		if ok, _ := path.Match(pattern, relPath); ok {
			return true
		}
		if ok, _ := path.Match(pattern, base); ok {
			return true
		}
		if isDir {
			trimmed := strings.TrimSuffix(pattern, "/")
			if trimmed == relPath || strings.HasPrefix(dirPrefix, trimmed+"/") {
				return true
			}
		}
	}
	return false
}

func cleanRemotePath(input string) string {
	input = filepath.ToSlash(strings.TrimSpace(input))
	if input == "" {
		return ""
	}
	if !strings.HasPrefix(input, "/") {
		input = "/" + input
	}
	return strings.TrimRight(input, "/")
}

func isSFTPNotExist(err error) bool {
	return os.IsNotExist(err) || strings.Contains(strings.ToLower(err.Error()), "no such file")
}
