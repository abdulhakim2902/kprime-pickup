FROM golang:alpine AS builder

ARG ACCESS_USER
ARG ACCESS_TOKEN

ENV GOPRIVATE=github.com/Undercurrent-Technologies/kprime-utilities

RUN apk add git

RUN git config --global url.https://${ACCESS_USER}:${ACCESS_TOKEN}@github.com/.insteadOf https://github.com

RUN mkdir /src
ADD . /src
WORKDIR /src

# RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux go build -o pickup main.go

FROM alpine:latest

RUN apk add --no-cache tzdata
ENV TZ=Asia/Singapore
ENV IMAGE=true

COPY --from=builder /src/pickup .

CMD ["./pickup"]