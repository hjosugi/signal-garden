FROM golang:1.26-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/signal-garden ./cmd/server

FROM scratch
WORKDIR /app
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/signal-garden /signal-garden
COPY --chown=65532:65532 config /app/config
COPY --chown=65532:65532 data /app/data
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/signal-garden"]
