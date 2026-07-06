FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/worker ./worker/cmd/server

FROM alpine:3.20
RUN apk add --no-cache ffmpeg ca-certificates
COPY --from=builder /out/worker /app/worker
WORKDIR /app
EXPOSE 8091 50052 2113
ENTRYPOINT ["/app/worker"]
