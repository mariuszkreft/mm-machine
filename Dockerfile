FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY main.go ./
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/mm-machine .

FROM alpine:3.20
WORKDIR /app
RUN adduser -D -H -u 10001 appuser && mkdir -p /app/data && chown appuser /app/data
COPY --from=build /out/mm-machine /app/mm-machine
ENV PORT=8080 DB_PATH=/app/data/mm.db LLM_BASE_URL=http://192.168.31.90:8000/v1 LLM_MODEL=deepseek-v4-flash-0731
EXPOSE 8080
VOLUME ["/app/data"]
USER appuser
ENTRYPOINT ["/app/mm-machine"]
