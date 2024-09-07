FROM alpine:3.20 as BUILDER
RUN apk add openjdk21-jdk --no-cache --repository=https://dl-cdn.alpinelinux.org/alpine/3.20/community
RUN jlink --add-modules java.base --output /jdk
FROM alpine:3.20
COPY --from=BUILDER /jdk jdk
WORKDIR /app
