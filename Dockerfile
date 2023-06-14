FROM golang:1.20-alpine as builder
RUN mkdir /build
COPY main.go /build/
COPY go.mod /build/
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -o app .

FROM alpine:latest
COPY --from=builder /build/app .
EXPOSE 8080
CMD ["./app"]
