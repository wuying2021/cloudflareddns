FROM golang:1-alpine AS builder

COPY . /go/src/github.com/wuying2021/cloudflareddns
WORKDIR /go/src/github.com/wuying2021/cloudflareddns

RUN go build -v -o CloudflareDDNS -trimpath -ldflags "-s -w -buildid="

FROM alpine
COPY --from=builder /go/src/github.com/wuying2021/cloudflareddns/CloudflareDDNS /usr/local/bin/CloudflareDDNS

WORKDIR /CloudflareDDNS

CMD ["/usr/local/bin/CloudflareDDNS"]
