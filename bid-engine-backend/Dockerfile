ARG CACHE_BUST
FROM golang:1.25-alpine AS build

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bid-engine cmd/server/main.go


FROM ubuntu:22.04

WORKDIR /service

ARG DEBIAN_FRONTEND=noninteractive
ENV TZ=Asia/Shanghai

# 文档转换（.doc/.docx → PDF）已拆到独立的 doc-converter 服务（见 docker-compose.yml），
# 本镜像不再需要 LibreOffice。这里只保留：
# - fonts-wqy-zenhei：image_processor.go 在图片上绘制中文时探测的字体
# - fontconfig：字体发现；tzdata/ca-certificates：时区与根证书
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      tzdata ca-certificates fontconfig \
      fonts-wqy-zenhei && \
    ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo "$TZ" > /etc/timezone && \
    rm -rf /var/lib/apt/lists/*

COPY conf/conf-container.yml conf/conf.yml
COPY conf/llm-config.yml conf/llm-config.yml

COPY --from=build /build/bid-engine ./

EXPOSE 8081

CMD ["./bid-engine"]
