FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod .
COPY go.sum .
COPY main.go .

COPY app/ app/

RUN go mod download

RUN CGO_ENABLED=0 GOFLAGS=-mod=mod GOOS=linux go build -ldflags="-w -s" -a -o /myauthapp .

FROM alpine AS final

USER nobody:nobody

COPY --chown=nobody:nobody --from=builder /myauthapp /myauthapp

EXPOSE 50052

CMD ["/myauthapp"]
