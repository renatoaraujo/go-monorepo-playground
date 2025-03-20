FROM golang:1.24 AS builder
ARG SERVICE

RUN if [ -z "$SERVICE" ]; then echo "Error: SERVICE argument is required"; exit 1; fi

WORKDIR /app

COPY ./$SERVICE/go.mod ./$SERVICE/go.sum ./
RUN go mod download

COPY ./$SERVICE .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/$SERVICE ./cmd/$SERVICE/main.go

FROM scratch AS runner
ARG SERVICE
COPY --from=builder /app/$SERVICE /service

CMD ["/service"]