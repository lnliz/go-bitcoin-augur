FROM golang:1.26-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
COPY internal/ internal/
COPY *.go ./

COPY example/go.mod example/go.sum example/
COPY example/*.go example/

WORKDIR /build/example
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o augur-example .

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /build/example/augur-example .

ENV BITCOIN_RPC_URL="http://host.docker.internal:8332"

EXPOSE 8080 9876

CMD ["./augur-example"]
