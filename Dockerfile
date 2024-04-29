FROM golang:alpine as builder

RUN mkdir /build

ADD . /build/

WORKDIR /build/api/

RUN go build -o main .

FROM alpine

RUN adduser -S -D -H -h /app appuser

USER appuser

COPY . /app

COPY --from=builder /build/api/main /app/

WORKDIR /app

EXPOSE 3000

CMD ["./main"]
