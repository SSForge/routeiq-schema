FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o auth-proxy .

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /src/auth-proxy /auth-proxy
EXPOSE 4317 4318
ENTRYPOINT ["/auth-proxy"]
