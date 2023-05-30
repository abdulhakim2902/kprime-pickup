FROM golang:1.19

ARG ACCESS_TOKEN

WORKDIR /go/src/app

COPY . .

RUN git config  --global url."https://oauth2:${ACCESS_TOKEN}@git.devucc.name".insteadOf "https://git.devucc.name"

RUN go get

RUN go build -o /pickup

CMD ["/pickup"]
