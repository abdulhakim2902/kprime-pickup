FROM alpine:latest

RUN apk add --no-cache tzdata
ENV TZ=Asia/Singapore

WORKDIR /

COPY ./pickup /pickup

CMD ["./pickup"]