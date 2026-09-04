FROM nginx:1.20-alpine
LABEL maintainer="jonsosnyan <https://jonssonyan.com>"
RUN mkdir -p /tpdata/trojan-panel-ui/
WORKDIR /tpdata/trojan-panel-ui/
ENV TZ=Asia/Shanghai
COPY dist/ .
ENTRYPOINT nginx -g 'daemon off;'