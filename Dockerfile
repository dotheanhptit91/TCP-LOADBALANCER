FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/${SERVICE}

FROM alpine:3.21
RUN apk add --no-cache conntrack-tools iproute2 nftables tcpdump \
    && addgroup -S app \
    && adduser -S -G app app
COPY --from=build /out/app /app
USER app
ENTRYPOINT ["/app"]
