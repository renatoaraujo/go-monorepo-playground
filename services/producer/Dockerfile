FROM golang:1.24 AS builder
ARG VERSION=dev

WORKDIR /app

COPY go.work .

COPY services/**/go.sum ./services/**/
COPY pkg/**/go.mod pkg/**/go.sum ./pkg/**/

COPY ./pkg ./pkg
COPY ./services ./services

RUN cd services/producer && CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-X main.version=$VERSION" \
    -o /app/bin/producer \
    ./...

FROM scratch

WORKDIR /app

COPY --from=builder /app/bin/* /app/

EXPOSE 8080

ENTRYPOINT ["/app/producer"]