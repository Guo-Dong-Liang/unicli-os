# UniCLI Base Runtime Image
FROM alpine:3.20

RUN apk add --no-cache \
    ca-certificates \
    tini \
    bash \
    curl \
    && rm -rf /var/cache/apk/*

# Default workdir
WORKDIR /workspace

# Run with tini to handle signals properly
ENTRYPOINT ["/sbin/tini", "--"]

# Default user is non-root
RUN adduser -D -h /workspace unicli
USER unicli
