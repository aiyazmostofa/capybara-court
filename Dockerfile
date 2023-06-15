FROM golang:1.20-alpine as GO_BUILDER
RUN mkdir /build
COPY main.go /build/
COPY go.mod /build/
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux go build -o app .

FROM alpine:latest as JDK_BUILDER
RUN apk add openjdk20-jdk --repository=http://dl-cdn.alpinelinux.org/alpine/edge/testing/
RUN jlink --add-modules java.base,jdk.compiler --output /jdk

FROM alpine:latest
RUN mkdir production
WORKDIR /production
COPY --from=GO_BUILDER /build/app app
COPY --from=JDK_BUILDER /jdk jdk
EXPOSE 8080
CMD ["./app"]
