FROM alpine:latest

WORKDIR /

COPY ./pickup /pickup

CMD ["./pickup"]