ARG GO_VERSION=1.26.3-alpine

FROM golang:${GO_VERSION} AS deps

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download && go mod verify

FROM deps AS dev

RUN go install github.com/air-verse/air@latest
EXPOSE 8080
CMD ["air", "-c", ".air.toml"]

FROM deps AS builder

COPY . .
RUN CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    go build \
    -ldflags="-w -s" \
    -trimpath \
    -o /app/image-processor \
    ./cmd/rest_api/main.go
RUN ! ldd /app/image-processor 2>&1 | grep -q "libc"


FROM aquasec/trivy:latest AS scanner

COPY --from=builder /app/image-processor /app/image-processor
RUN trivy fs \
    --no-progress \
    --exit-code 0 \
    --severity HIGH,CRITICAL \
    /app/image-processor


FROM gcr.io/distroless/static-debian12:nonroot AS final

COPY --from=builder /app/image-processor /image-processor
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/image-processor"]
