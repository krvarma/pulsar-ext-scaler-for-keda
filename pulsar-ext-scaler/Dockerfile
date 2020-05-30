FROM golang:alpine AS builder
RUN mkdir /app
ADD . /app/
WORKDIR /app
RUN go build -o pulsar-ext-scaler .

FROM alpine
RUN mkdir /app
WORKDIR /app
COPY --from=builder /app/pulsar-ext-scaler .

CMD ["./pulsar-ext-scaler"]