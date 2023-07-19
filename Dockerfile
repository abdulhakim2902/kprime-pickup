FROM golang:alpine AS builder

RUN mkdir /src
ADD . /src
WORKDIR /src

RUN CGO_ENABLED=0 GOOS=linux go build -o pickup main.go

FROM alpine:latest

RUN apk add --no-cache tzdata
ENV TZ=Asia/Singapore

COPY --from=builder /src/pickup .

CMD ["./pickup"]