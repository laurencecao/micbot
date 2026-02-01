#!/bin/bash

set -e

CERT_DIR="${1:-./certs}"
HOSTNAME="${2:-}"
CERT_FILE="${CERT_DIR}/cert.pem"
KEY_FILE="${CERT_DIR}/key.pem"

mkdir -p "$CERT_DIR"

echo "正在生成自签名证书..."
echo "证书目录: $CERT_DIR"

if ! command -v mkcert &> /dev/null; then
    echo "mkcert 未安装，正在安装..."
    go install filippo.io/mkcert@latest
    
    if ! command -v mkcert &> /dev/null; then
        echo "错误: mkcert 安装失败，请确保 Go 已正确安装并配置 GOPATH"
        exit 1
    fi
fi

echo "初始化 mkcert 本地 CA..."
mkcert -install

echo "生成证书和密钥..."
cd "$CERT_DIR"

if [ -n "$HOSTNAME" ]; then
    echo "包含主机名: localhost, 127.0.0.1, ::1, $HOSTNAME"
    mkcert -cert-file cert.pem -key-file key.pem localhost 127.0.0.1 ::1 "$HOSTNAME"
else
    echo "包含主机名: localhost, 127.0.0.1, ::1"
    mkcert -cert-file cert.pem -key-file key.pem localhost 127.0.0.1 ::1
fi

echo ""
echo "证书生成成功！"
echo "证书文件: $CERT_FILE"
echo "密钥文件: $KEY_FILE"
echo ""
echo "请在 config.ini 中配置以下路径："
echo "cert_file = $CERT_FILE"
echo "key_file = $KEY_FILE"
