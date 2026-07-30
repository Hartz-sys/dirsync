package sshauth

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	appconfig "dirsync/internal/config"

	"golang.org/x/crypto/ssh"
)

func NewClientConfig(cfg *appconfig.Config) (*ssh.ClientConfig, error) {
	authMethod, err := authMethod(cfg)
	if err != nil {
		return nil, err
	}

	return &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}, nil
}

func Dial(cfg *appconfig.Config) (*ssh.Client, error) {
	clientConfig, err := NewClientConfig(cfg)
	if err != nil {
		return nil, err
	}

	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
	client, err := ssh.Dial("tcp", addr, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("连接远端失败 %s: %w", addr, err)
	}
	return client, nil
}

func authMethod(cfg *appconfig.Config) (ssh.AuthMethod, error) {
	switch cfg.ResolvedAuthMethod() {
	case appconfig.AuthMethodKey:
		signer, err := signerFromPrivateKey(cfg.PrivateKey, cfg.PrivateKeyPassphrase)
		if err != nil {
			return nil, err
		}
		return ssh.PublicKeys(signer), nil
	case appconfig.AuthMethodPassword:
		if strings.TrimSpace(cfg.Password) == "" {
			return nil, fmt.Errorf("未配置 password")
		}
		return ssh.Password(cfg.Password), nil
	default:
		return nil, fmt.Errorf("未配置可用认证方式")
	}
}

func signerFromPrivateKey(path, passphrase string) (ssh.Signer, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取私钥失败: %w", err)
	}

	if strings.TrimSpace(passphrase) != "" {
		signer, err := ssh.ParsePrivateKeyWithPassphrase(key, []byte(passphrase))
		if err != nil {
			return nil, fmt.Errorf("解析带密码私钥失败: %w", err)
		}
		return signer, nil
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %w", err)
	}
	return signer, nil
}
