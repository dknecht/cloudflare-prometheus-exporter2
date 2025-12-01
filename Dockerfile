FROM golang:alpine as builder

WORKDIR /app

# Copy go.mod and go.sum to download dependencies
COPY go.mod go.sum ./

RUN go mod download

# Copy the entire source code
COPY . .


RUN CGO_ENABLED=0 GOOS=linux go build --ldflags '-w -s -extldflags "-static"' -o cloudflare_exporter ./cmd

FROM alpine:3.19

RUN apk update && apk add ca-certificates

COPY --from=builder /app/cloudflare_exporter cloudflare_exporter

ENV CF_API_TOKEN ""

ENTRYPOINT [ "./cloudflare_exporter" ]
