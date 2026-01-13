FROM golang:1.24 AS build

RUN useradd -u 10001 dimo

WORKDIR /go/src/github.com/DIMO-Network/meta-transaction-processor/
COPY . /go/src/github.com/DIMO-Network/meta-transaction-processor/

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOFLAGS=-mod=vendor

RUN ls
RUN go mod tidy
RUN go mod vendor
RUN make install

FROM busybox AS package

LABEL maintainer="DIMO <hello@dimo.zone>"

WORKDIR /

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /go/src/github.com/DIMO-Network/meta-transaction-processor/target/bin/meta-transaction-processor .
COPY --from=build /go/src/github.com/DIMO-Network/meta-transaction-processor/migrations ./migrations

USER dimo

EXPOSE 8080
EXPOSE 8888

CMD /meta-transaction-processor
