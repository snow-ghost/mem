FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY go.mod ./
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o mem ./cmd/mem/

FROM scratch
COPY --from=builder /build/mem /mem
WORKDIR /project
ENTRYPOINT ["/mem"]
