#FROM alpine:3.4
FROM ubuntu:cosmic
MAINTAINER william <xxx@xxx.com>
RUN mkdir /dns-proxy
WORKDIR /dns-proxy
#ADD https://github.com/jursomo/go-dns-proxy/releases/download/1.0.0/go-dns-proxy_linux-amd64 /dns-proxy

COPY go-dns-proxy /dns-proxy/
COPY config.toml /dns-proxy/config.toml
RUN chmod +x /dns-proxy/go-dns-proxy

#docker run -p 53:53/udp -it --name mydnsproxy godnsproxy-ubuntu
EXPOSE 53 53/udp
ENTRYPOINT ["/dns-proxy/go-dns-proxy -c config.toml &"]
