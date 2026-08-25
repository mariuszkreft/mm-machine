FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
COPY static ./static
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/mm-machine .

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/mm-machine /app/mm-machine
ENV PORT=8080
EXPOSE 8080
RUN adduser -D -H -u 10001 appuser
USER appuser
ENTRYPOINT ["/app/mm-machine"]
