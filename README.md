# SkillHub CLI

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

SkillHub CLI 是一个用于发现、安装和管理 AI 技能（Skills）的命令行工具。它同时支持社区注册表（Community Registry）和企业私有注册表（Enterprise Registry），让你和你的团队可以轻松共享和管理 AI 技能。

## 为什么用 Go 重写？

这个项目最初有一个 Python 实现。在实际使用中，我们发现了一些可以改进的地方：

- **运行时依赖**：Python 版本要求 Python 3，而部分环境中预装的 Python 3.6 等较低版本无法直接运行，增加了用户的环境准备成本。
- **命令行体验**：原有的 argparse 实现缺少结构化的子命令帮助信息和 shell 自动补全，新用户需要仔细翻看命令信息才能了解完整用法。
- **分发便利性**：Go 可以编译为单个静态二进制文件，无需安装运行时依赖，`curl` 下载即可用。即便是直接 `go install` 相比python也简化很多。

基于这些考虑，我们用 Go 从头重写了一个更友好、更专业的 CLI 工具，同时保持与现有 SkillHub 生态（包括 OpenClaw）的完全兼容。社区和企业的 skill 数据格式保持一致，两边都可以正常搜索和安装。

> 📝 整个重写、测试和优化工作大约耗时 **1.5 小时**。

## 功能概览

| 命令                                    | 说明                         |
| --------------------------------------- | ---------------------------- |
| `skillhub search [关键词]`            | 搜索社区和企业注册表中的技能 |
| `skillhub install <slug>`             | 安装技能到本地目录           |
| `skillhub list`                       | 列出已安装的技能             |
| `skillhub upgrade [slug]`             | 升级已安装的技能             |
| `skillhub publish <路径>`             | 发布技能到社区注册表         |
| `skillhub login --key <api-key>`      | 登录企业注册表               |
| `skillhub logout`                     | 登出企业注册表               |
| `skillhub auth login --token <token>` | 登录社区发布账号             |
| `skillhub auth whoami`                | 查看当前登录用户             |
| `skillhub config`                     | 查看和管理配置               |
| `skillhub version`                    | 显示 CLI 版本                |
| `skillhub completion <shell>`         | 生成 shell 自动补全脚本      |

## 快速开始

### 安装

#### 从源码编译

```bash
git clone https://github.com/hitzhangjie/skillhub.git
cd skillhub
make build        # 编译到当前目录
make install      # 编译并安装到 $GOPATH/bin
```

#### 直接下载二进制（推荐）

```bash
# 下载最新版本（替换为实际发布地址）
curl -L -o skillhub https://github.com/hitzhangjie/skillhub/releases/latest/download/skillhub-linux-amd64
chmod +x skillhub
sudo mv skillhub /usr/local/bin/
```

### 启用 Shell 自动补全

```bash
# Bash
source <(skillhub completion bash)

# Zsh
source <(skillhub completion zsh)

# Fish
skillhub completion fish | source
```

永久生效（以 bash 为例）：

```bash
echo 'source <(skillhub completion bash)' >> ~/.bashrc
```

### 基本用法

**搜索技能：**

```bash
# 查看所有可用技能
skillhub search

# 按关键词搜索
skillhub search "code review"

# JSON 格式输出（适合脚本调用）
skillhub search --json "code review"
```

**安装技能：**

```bash
# 安装社区技能
skillhub install my-skill

# 安装企业技能（需要先登录）
skillhub install @my-org/private-skill

# 安装指定版本
skillhub install @my-org/private-skill@1.0.0

# 强制覆盖已安装的版本
skillhub install --force my-skill
```

**管理已安装的技能：**

```bash
# 列出已安装的技能
skillhub list

# 升级所有技能
skillhub upgrade

# 检查可用的升级（不安装）
skillhub upgrade --check-only
```

**企业注册表：**

```bash
# 登录
skillhub login --key sk-ent-abc123

# 登录（自定义 host）
skillhub login --key sk-ent-abc123 --host https://custom.skillhub.cn

# 查看已配置的企业源
skillhub config

# 登出
skillhub logout
```

**发布技能：**

```bash
# 验证元数据（不实际上传）
skillhub publish ./my-skill --dry-run

# 正式发布
skillhub publish ./my-skill --version 1.2.0 --changelog "bug fixes"
```

## 目录结构

```
skillhub/
├── main.go                      # 入口，设置版本号
├── cmd/                         # CLI 命令定义（基于 cobra）
│   ├── root.go                  # 根命令和全局 flag
│   ├── search.go                # search 命令
│   ├── install.go               # install 命令
│   ├── list.go                  # list 命令
│   ├── upgrade.go               # upgrade 命令
│   ├── publish.go               # publish 命令
│   ├── login.go                 # login 命令（企业）
│   ├── logout.go                # logout 命令（企业）
│   ├── auth.go                  # auth 子命令组（社区 token 管理）
│   ├── config.go                # config 命令
│   └── version.go               # version 命令
├── pkg/                         # 核心逻辑包
│   ├── community/               # 社区注册表搜索
│   ├── config/                  # 配置和常量
│   ├── credentials/             # 凭证管理
│   ├── enterprise/              # 企业注册表 API
│   ├── index/                   # 技能索引加载与解析
│   ├── install/                 # 下载、校验、解压
│   ├── lockfile/                # 安装锁文件
│   ├── metadata/                # SKILL.md 元数据解析
│   ├── upgrade/                 # 升级逻辑
│   └── version/                 # 语义版本比较
├── Makefile                     # 构建脚本
├── go.mod
└── go.sum
```

## 配置

### 环境变量

| 变量                     | 说明                       |
| ------------------------ | -------------------------- |
| `SKILLHUB_ORG`         | 企业组织标识               |
| `SKILLHUB_API_KEY`     | 企业 API Key               |
| `SKILLHUB_HOST`        | 企业注册表地址             |
| `SKILLHUB_TOKEN`       | 个人 API Token（社区发布） |
| `SKILLHUB_SECRET`      | 企业 API Key（直接下载）   |
| `SKILLHUB_SEARCH_URL`  | 搜索 API 地址              |
| `SKILLHUB_CONFIG_PATH` | 配置文件路径               |

### 凭证存储

登录后的凭证存储在 `~/.skillhub/credentials.json`。请注意这是一个未加密的本地文件，建议配置凭证助手（credential helper）来增强安全性。

### 自定义索引地址

```bash
# 使用本地索引文件
skillhub search --index ./my-skills.json

# 使用远程索引
skillhub search --index https://example.com/skills.json
```

## 安全特性

- **路径穿越防护**：zip 解压时检测并拒绝 `../` 和绝对路径
- **SHA256 校验**：下载后自动校验文件完整性
- **Zip bomb 防护**：发布上传时限制解压后总大小不超过 50MB
- **安全凭证提示**：首次登录时明确告知凭证存储位置

## 技术栈

- [Cobra](https://github.com/spf13/cobra) — CLI 框架，提供子命令、帮助信息、shell 补全
- Go 1.26+ 标准库 — 零外部依赖的核心功能（HTTP、JSON、ZIP、Tar）

## License

MIT

---

🤖 本工具由 [hitzhangjie](https://github.com/hitzhangjie) 用 Go 重写，开发耗时约 1.5 小时。
