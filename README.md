# dirsync

`dirsync` 是一个运行在 macOS 上的命令行工具，用于把本地目录通过 SFTP 同步到 Linux 服务器目录，支持密码和 SSH 私钥两种认证方式。

## 功能

- 递归同步本地目录到远端目录
- 同步前自动备份远端目录（SSH `cp -a`）
- 支持密码认证和 SSH 私钥认证
- 支持排除规则
- 支持 `dry_run` 预演模式
- 支持删除远端多余文件（镜像模式）
- 支持 `check` 检查配置与连通性

## 构建

```bash
go mod tidy
go build -o dirsync .
```

## 初始化配置

```bash
./dirsync init
./dirsync init -c ~/.dirsync/config.yaml
```

默认会根据内置示例配置生成配置文件。

## 配置示例

参考 `config.example.yaml`：

```yaml
name: default
host: 192.168.1.100
port: 22
user: deploy
auth_method: auto   # password / key / auto
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

backup_before_sync: true
# backup_dir: "/opt/backups"
```

说明：

- `auth_method` 可选：`password`（密码）、`key`（私钥）、`auto`（默认；有 `private_key` 用密钥，否则用密码）
- 也可写别名：`private_key` / `ssh_key` 等同 `key`，`pwd` / `pass` 等同 `password`
- 如果私钥带密码，可填写 `private_key_passphrase`
- `backup_before_sync` 默认开启；设为 `false` 可关闭
- `backup_dir` 为空时，备份到同级目录，例如 `/opt/app.bak.20260727152030`
- `backup_dir` 有值时，备份到该目录下，例如 `/opt/backups/app-20260727152030`
- 真实 `config.yaml` 已被 `.gitignore` 忽略，避免误提交密码

## 使用方法

### 交互模式（推荐）

直接运行，按提示选择目标和操作：

```bash
./dirsync
```

#### 桌面双击运行（macOS）

1. 使用项目里的 [`start_sync.command`](start_sync.command)，或桌面上的同名文件
2. 首次若提示无法打开：右键 → 打开，或在「系统设置 → 隐私与安全性」允许
3. 双击后会打开终端，进入交互选择菜单

也可在终端手动执行：

```bash
~/Desktop/start_sync.command
```

菜单会自动扫描：

- 当前目录：`config.yaml`、`config.*.yaml`、`*.sync.yaml`
- 用户目录：`~/.dirsync/` 下同类文件

可在配置里加 `name` 作为菜单显示名，例如：

```yaml
name: itsm-prod
host: 10.0.0.5
...
```

多目标示例：

```bash
config.dev.yaml
config.prod.yaml
# 或
dev.sync.yaml
prod.sync.yaml
```

### 命令行模式

```bash
# 检查配置与连接
./dirsync check -c config.yaml

# 按配置同步（会先备份远端目录）
./dirsync sync -c config.yaml

# 临时覆盖本地和远端目录
./dirsync sync -c config.yaml --local ./src --remote /opt/app
```

## 同步规则

- 同步开始前，若远端目录已存在，会先完整复制一份备份
- 本地目录递归遍历，目录和文件都应用 `exclude` 规则
- 文件大小不同或修改时间相差超过 1 秒时，会重新上传
- 上传时先写入远端临时文件，再重命名，减少半截文件风险
- `delete_extra: true` 时，会删除远端多出的文件并清理空目录
- `dry_run: true` 时只打印计划操作（含备份路径），不实际修改远端

## 安全说明

当前版本默认使用 `InsecureIgnoreHostKey()`，适合内网或快速落地场景。生产环境如果需要更严格校验，后续可扩展为 `known_hosts` 校验。
