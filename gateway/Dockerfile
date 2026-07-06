FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/gateway ./gateway/cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /out/gateway /app/gateway
WORKDIR /app
EXPOSE 8080 50051 2112
ENTRYPOINT ["/app/gateway"]
