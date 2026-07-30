package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	appconfig "dirsync/internal/config"
)

func runInteractive() error {
	candidates, err := appconfig.ListCandidates()
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return fmt.Errorf("未找到可用配置。可先执行: ./dirsync init  或创建 config.dev.yaml / config.prod.yaml")
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("dirsync - 请选择同步目标")
	fmt.Println()
	for i, item := range candidates {
		fmt.Printf("  %d) %s\n", i+1, item.Summary())
		fmt.Printf("     配置: %s\n", item.Path)
		if item.LocalDir != "" {
			fmt.Printf("     本地: %s\n", item.LocalDir)
		}
	}
	fmt.Println("  0) 退出")
	fmt.Println()

	idx, err := promptChoice(reader, "请输入编号", 0, len(candidates))
	if err != nil {
		return err
	}
	if idx == 0 {
		fmt.Println("已取消")
		return nil
	}

	selected := candidates[idx-1]
	fmt.Println()
	fmt.Println("请选择操作:")
	fmt.Println("  1) 同步 (sync)")
	fmt.Println("  2) 检查连接 (check)")
	fmt.Println("  0) 取消")
	fmt.Println()

	action, err := promptChoice(reader, "请输入编号", 0, 2)
	if err != nil {
		return err
	}
	switch action {
	case 0:
		fmt.Println("已取消")
		return nil
	case 1:
		fmt.Printf("\n开始同步: %s\n\n", selected.Summary())
		return runSync([]string{"-c", selected.Path})
	case 2:
		fmt.Printf("\n开始检查: %s\n\n", selected.Summary())
		return runCheck([]string{"-c", selected.Path})
	default:
		return fmt.Errorf("无效选择")
	}
}

func promptChoice(reader *bufio.Reader, label string, min, max int) (int, error) {
	for {
		fmt.Printf("%s [%d-%d]: ", label, min, max)
		line, err := reader.ReadString('\n')
		if err != nil {
			return 0, fmt.Errorf("读取输入失败: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			fmt.Println("请输入有效编号")
			continue
		}
		n, err := strconv.Atoi(line)
		if err != nil || n < min || n > max {
			fmt.Printf("请输入 %d 到 %d 之间的数字\n", min, max)
			continue
		}
		return n, nil
	}
}
