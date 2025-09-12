FROM golang:1.23-alpine3.20 as builder

WORKDIR /root

COPY . ./

RUN go build -o bin/payments-read-model cmd/main.go

FROM alpine:3.20

# Allow customization of user ID and group ID (it's useful when you use Docker bind mounts)
ARG UID=1000
ARG GID=1000

RUN addgroup -g ${GID} -S app && adduser -u ${UID} -S -G app app

WORKDIR /home/app

COPY --from=builder /root/bin/payments-read-model ./


RUN chown app:app ./payments-read-model
RUN chmod +x ./payments-read-model

USER app

CMD ["./payments-read-model"]
