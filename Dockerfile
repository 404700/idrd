# 第一阶段：构建前端
FROM node:20-alpine AS frontend-builder
WORKDIR /web
COPY web/package.json ./
# 如果有 package-lock.json 也应该复制
RUN npm install
COPY web/ .
# 构建前端，产物在 /web/dist
RUN npm run build

# 第二阶段：构建后端
FROM golang:1.25-alpine AS backend-builder

# 安装编译依赖（SQLite 需要 CGO 和 gcc）
RUN apk add --no-cache git gcc musl-dev

WORKDIR /app

# 复制 go.mod 和 go.sum 并下载依赖
COPY go.mod go.sum* ./
RUN go mod download

# 复制源代码
COPY . .



# 准备静态文件目录
RUN mkdir -p server/static

# 从前端构建阶段复制静态文件到 server/static
COPY --from=frontend-builder /web/dist/ /app/server/static/

# 再次 tidy 确保依赖完整
RUN go mod tidy && go mod download

# 构建二进制文件（启用 CGO 以支持 SQLite）
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-linkmode external -extldflags "-static"' -o idrd ./cmd/idrd

# 第三阶段：运行时
# 选项1：Alpine（支持健康检查，推荐）
FROM alpine:3.23
RUN apk add --no-cache wget ca-certificates tzdata

# 选项2：Distroless（更小体积，但无健康检查支持）
# FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=backend-builder /app/idrd /app/idrd



# 创建数据目录（配置文件存储）
VOLUME /data


# 暴露端口
EXPOSE 8080

# 设置环境变量
ENV CONFIG_PATH=/data/config.yaml

# 注意：不在此处设置 USER，通过 docker-compose.yml 的 user 参数指定
# 这样可以确保与初始化时创建文件的用户一致

# 入口点
ENTRYPOINT ["/app/idrd"]
