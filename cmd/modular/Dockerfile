# 模块化部署镜像：一个模块一个进程，用 MODULE 参数选 cmd/modular 下的入口。
# 四个模块统一监听 8080(见 configs/<module>.yaml)，容器间靠网络命名空间隔离，
# 对外端口由 compose 的 ports / nginx upstream 区分。
#
# 构建上下文必须是仓库根目录(要 go.mod / pkg / internal)：
#
#   docker build -f cmd/modular/Dockerfile --build-arg MODULE=system -t ruoyi-go:system .
#   docker build -f cmd/modular/Dockerfile --build-arg MODULE=auth   -t ruoyi-go:auth .
#
# 同一模块起多副本时，configs/<module>.yaml 里的 snowflake.workerId 各进程必须
# 互不相同，否则共用一个库会撞主键。副本要挂载各自的 yaml
# (-v ./configs/system-2.yaml:/app/configs/system.yaml:ro)，环境变量无效
# —— pkg/config 没开 viper.AutomaticEnv。

FROM golang:1.26-alpine AS builder

ARG MODULE
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 空 MODULE 会编译出仓库里第一个 main 包这类意外产物，先挡掉。
# CGO 关掉：依赖里没有 cgo(MySQL 走纯 Go 驱动)，静态链接才能放进 alpine 直接跑。
RUN test -d ./cmd/modular/${MODULE} || { echo "MODULE 无效: '${MODULE}'"; exit 1; } \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /app/server ./cmd/modular/${MODULE}

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

# /ping 免鉴权，不碰 DB/Redis，纯活性探针。四个模块的 InitRouter 都以空前缀
# 注册路由，故探针路径一致。
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/ping || exit 1

# 工作目录必须是 /app：config.Load 传的是相对路径 configs/*.yaml。
ENTRYPOINT ["/app/server"]
