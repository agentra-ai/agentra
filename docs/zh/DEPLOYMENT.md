# Agentra 部署与发布指南

Agentra 支持两条交付路径：

- 推送版本 tag 后，将可验证的 CLI 资产发布到 GitHub Releases，并把多架构镜像发布到 GHCR。
- 自托管用户也可以用 Docker Compose 从同一份源码本地构建。

本文描述的软件供应链能力会从下一个 `v*` tag 开始公开生效。此前创建的 release 可能还没有 SBOM、Sigstore bundle、GHCR 镜像或 GitHub attestation。

## Tag 发布流水线

推送语义版本 tag 会并行触发两个 workflow：

```text
v0.6.0
  ├─ release.yml
  │    ├─ Darwin/Linux/Windows CLI 归档（amd64 + arm64）
  │    ├─ 覆盖归档、SBOM、安装器的 SHA-256
  │    ├─ 每个归档一份 SPDX 2.3 SBOM
  │    ├─ checksums 与 SBOM 的 Cosign keyless bundle
  │    ├─ GitHub build provenance attestation
  │    └─ Homebrew Cask 更新
  └─ docker.yml
       ├─ server、gateway、web 镜像
       ├─ 每个 tag 的 linux/amd64 + linux/arm64 manifest
       ├─ BuildKit SBOM 与 max-mode provenance
       ├─ 每个镜像 digest 的 Cosign keyless 签名
       └─ 写入 registry 的 GitHub provenance attestation
```

仅在仓库检查通过后创建 tag：

```bash
make check
git tag v0.6.0
git push origin v0.6.0
```

容器 workflow 使用仓库范围的 `GITHUB_TOKEN`，不再需要个人 Docker Hub 凭据。唯一的跨仓库 secret 是 `HOMEBREW_TAP_GITHUB_TOKEN`，其权限只需覆盖 `agentra-ai/homebrew-tap`。

### 镜像标签

三个组件共用官方 package `ghcr.io/agentra-ai/agentra`：

```bash
docker pull ghcr.io/agentra-ai/agentra:server-v0.6.0
docker pull ghcr.io/agentra-ai/agentra:gateway-v0.6.0
docker pull ghcr.io/agentra-ai/agentra:web-v0.6.0

# 稳定版本同时提供滚动别名。
docker pull ghcr.io/agentra-ai/agentra:server-v0.6
docker pull ghcr.io/agentra-ai/agentra:server-latest
```

每个标签都是包含 `linux/amd64` 与 `linux/arm64` 的多平台 manifest。

## 验证 CLI release

从官方渠道安装 Cosign 与 GitHub CLI，然后从同一 release 下载归档、checksum 和 checksum bundle。验证时把 workflow 身份固定到本仓库与当前 tag：

```bash
VERSION=v0.6.0
ASSET=agentra_linux_amd64.tar.gz
BASE="https://github.com/agentra-ai/agentra/releases/download/$VERSION"

curl -fLO "$BASE/$ASSET"
curl -fLO "$BASE/checksums.txt"
curl -fLO "$BASE/checksums.txt.sigstore.json"

cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity "https://github.com/agentra-ai/agentra/.github/workflows/release.yml@refs/tags/$VERSION" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt

grep "  $ASSET$" checksums.txt | shasum -a 256 -c -
gh attestation verify "$ASSET" -R agentra-ai/agentra
```

每个归档还有 `${ASSET}.spdx.json` SBOM 与 `${ASSET}.spdx.json.sigstore.json` bundle；检查依赖清单前，使用同一 workflow 身份执行 `cosign verify-blob`。

Shell 与 PowerShell 安装器始终强制执行 SHA-256 校验。Cosign 和 GitHub provenance 需要显式验证，因为一台干净机器无法从“正在被验证的产物”安全地自举独立验证工具。

## 验证容器镜像

```bash
VERSION=v0.6.0
IMAGE="ghcr.io/agentra-ai/agentra:server-$VERSION"

docker pull "$IMAGE"

cosign verify "$IMAGE" \
  --certificate-identity "https://github.com/agentra-ai/agentra/.github/workflows/docker.yml@refs/tags/$VERSION" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com

gh attestation verify "oci://$IMAGE" -R agentra-ai/agentra
docker buildx imagetools inspect "$IMAGE"
```

BuildKit SBOM 与 provenance 会附着在 OCI image index 上。审计时应记录标签解析出的 digest，不能把可变 tag 本身当作不可变证据。

## Docker Compose 自托管

前置条件：Docker 24+、Docker Compose v2、至少 4 GB RAM。

```bash
git clone https://github.com/agentra-ai/agentra.git
cd agentra

# 分别生成 PostgreSQL、JWT、MinIO 凭据，并以 0600 保存。
./scripts/bootstrap-env.sh

# 启动前检查公开 URL 与可选集成。
nano .env
docker compose up -d --build
```

默认 profile 让 PostgreSQL 与 MinIO 仅在内部网络可见，Web/API 只绑定 loopback，也不会启动 Adminer 或挂载 Docker socket 的 gateway。只有明确需要这些高权限入口时，才使用 `--profile debug` 或 `--profile cloud-runtime`。

验证部署：

```bash
curl http://127.0.0.1:8080/livez
curl http://127.0.0.1:8080/readyz
open http://127.0.0.1:3000
docker compose run --rm migrate
```

在宿主机单独安装 daemon，并连接自托管服务：

```bash
curl -fsSLO https://raw.githubusercontent.com/agentra-ai/agentra/main/scripts/install.sh
sh install.sh
rm install.sh
agentra setup --deployment self-host
```

Windows 用户运行 `scripts/install.ps1`；Homebrew 用户运行 `brew install --cask agentra-ai/tap/agentra`。

## Secret 参考

| Secret | 用途 | 来源 |
|---|---|---|
| `JWT_SECRET` | API 认证 | `scripts/bootstrap-env.sh` |
| `POSTGRES_PASSWORD` | PostgreSQL | `scripts/bootstrap-env.sh` |
| `MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY` | 对象存储 | `scripts/bootstrap-env.sh` |
| `RESEND_API_KEY` | `EMAIL_PROVIDER=resend` 时的邮件 OTP | Resend 账户 |
| `SMTP_PASSWORD` | `EMAIL_PROVIDER=smtp` 且开启认证时的邮件 OTP | SMTP 服务商 |
| `GOOGLE_CLIENT_*` | 可选 OAuth | Google Cloud Console |
| `HOMEBREW_TAP_GITHUB_TOKEN` | 仅 release workflow | 只授权 tap 仓库的 fine-grained token |

GHCR 发布、Cosign keyless 签名与 GitHub attestation 使用短期 workflow OIDC 和 `GITHUB_TOKEN`，不需要保存长期签名私钥。

公网部署应设置 `APP_ENV=production`，将 `EMAIL_PROVIDER` 配为 `resend` 或
`smtp`，并把 `EMAIL_FROM` 设为已验证域名下的发件人。SMTP 支持默认的
`SMTP_TLS_MODE=starttls`、隐式 TLS 的 `tls`，以及仅用于无认证内网 relay
的 `none`。可通过 `AGENTRA_SIGNUP_DISABLED`、`AGENTRA_SIGNUP_ALLOWLIST`
和 `AGENTRA_WORKSPACE_CREATION_DISABLED` 限制注册与 workspace 创建；关闭
公开注册后，预先邀请的用户仍可登录。

## 当前安全边界

- 从下一个 tag 开始，release 归档、checksums、SBOM 与 OCI digest 都有绑定 workflow 身份的供应链签名/attestation。
- macOS code signing/notarization 与 Windows Authenticode 属于另外的平台信任体系，目前尚未实现。
- 发布验证能够证明 workflow 身份与产物完整性，但不能替代对 tag 源码的审查或运行时加固。
