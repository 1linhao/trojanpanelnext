FROM alpine:3.15
LABEL maintainer="jonsosnyan <https://jonssonyan.com>"
WORKDIR /tpdata/trojan-panel-core/
ENV mariadb_ip=127.0.0.1 \
    mariadb_port=9507 \
    mariadb_user=root \
    mariadb_pas= \
    database=trojan_panel_db \
    account_table=account \
    redis_host=127.0.0.1 \
    redis_port=6378 \
    redis_pass= \
    crt_path=/tpdata/trojan-panel-core/cert/trojan-panel-core.crt \
    key_path=/tpdata/trojan-panel-core/cert/trojan-panel-core.key \
    grpc_tls_mode=legacy \
    grpc_client_ca_path=/tpdata/trojan-panel-core/pki/client-ca.crt \
    TP_KERNEL_RUNTIME=/tpdata/trojan-panel-core/runtime \
    grpc_port=8100 \
    server_port=8082 \
    TZ=Asia/Shanghai \
    GIN_MODE=release
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
COPY build/trojan-panel-core-${TARGETOS}-${TARGETARCH}${TARGETVARIANT} trojan-panel-core
COPY build/xray-${TARGETOS}-${TARGETARCH}${TARGETVARIANT} bin/xray/xray
COPY build/naiveproxy-${TARGETOS}-${TARGETARCH}${TARGETVARIANT} bin/naiveproxy/naiveproxy
COPY build/hysteria2-${TARGETOS}-${TARGETARCH}${TARGETVARIANT} bin/hysteria2/hysteria2
# Set apk China mirror
# RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
RUN apk add bash tzdata ca-certificates && \
    rm -rf /var/cache/apk/*
RUN chmod 755 ./trojan-panel-core bin/xray/xray bin/naiveproxy/naiveproxy bin/hysteria2/hysteria2 && \
    mkdir -p runtime
ENTRYPOINT ["./trojan-panel-core"]
