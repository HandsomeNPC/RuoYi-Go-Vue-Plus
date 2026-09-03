# 单体部署镜像：auth + system + monitor + resource 同进程，监听 :8080。
#
# 构建上下文必须是仓库根目录(要 go.mod / pkg / internal)，故 -f 指到本文件：
#   docker build -f cmd/standalone/Dockerfile -t ruoyi-go:standalone .
#   docker run -p 8080:8080 ruoyi-go:standalone
#
# configs 已打进镜像，改配置有两种方式：挂载覆盖整个目录
#   -v ./configs:/app/configs:ro
# 或只覆盖某一个文件。环境变量改不了配置 —— pkg/config 没开 viper.AutomaticEnv。

FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO 关掉：依赖里没有 cgo(MySQL 走纯 Go 驱动)，静态链接才能放进 alpine 直接跑。
# -trimpath 去掉构建机绝对路径，-s -w 砍掉符号表(内嵌了 10MB 的 ip2region 库，能省一点是一点)。
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /app/server ./cmd/standalone

FROM alpine:3.20

# ca-certificates：对象存储 / 阿里云短信 / 三方登录都是 HTTPS 出站调用。
# tzdata：DSN 用了 loc=Local，容器缺 tzdata 会按 UTC 写库，时间字段整体差 8 小时。
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 ruoyi
ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=builder /app/server /app/server
COPY configs /app/configs

USER ruoyi

EXPOSE 8080

# /system/ping 免鉴权(见 internal/system/router.go)，不碰 DB/Redis，纯活性探针。
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/system/ping || exit 1

# 工作目录必须是 /app：config.Load 传的是相对路径 configs/*.yaml。
ENTRYPOINT ["/app/server"]
