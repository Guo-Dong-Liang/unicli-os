FROM alpine:3.20

# Minimal runtime for protobuf pipe test CLI
RUN apk add --no-cache bash coreutils

# Create workspace
WORKDIR /workspace
RUN mkdir -p /app

# Entrypoint script
COPY say.sh /app/say.sh
RUN chmod +x /app/say.sh && chown -R nobody:nogroup /app /workspace

# Default user
USER nobody:nogroup

ENTRYPOINT ["/app/say.sh"]
