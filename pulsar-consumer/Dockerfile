FROM golang:alpine AS builder
RUN mkdir /app
ADD . /app/
WORKDIR /app
RUN go build -o consumer .

FROM alpine
RUN mkdir /app
WORKDIR /app
COPY --from=builder /app/consumer .

ENV PULSAR_SERVER pulsar://ubuntuserver:6650
ENV PULSAR_TOPIC my-topic
ENV PULSAR_SUBSCRIPTION_NAME first-subscription

CMD ["./consumer"]