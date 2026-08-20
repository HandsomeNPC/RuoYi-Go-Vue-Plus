# 多阶段构建，通过 MODULE 参数选择进程(auth / system)。
FROM golang:1.26-alpine AS builder
ARG MODULE
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/server ./cmd/${MODULE}

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/server /app/server
ENTRYPOINT ["/app/server"]
