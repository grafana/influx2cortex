FROM alpine:3.12

RUN addgroup -g 1000 app && \
  adduser -u 1000 -h /app -G app -S app
WORKDIR /app
USER app

ARG cmd

COPY "dist/${cmd}" "/app/${cmd}"

# build args are only available during build time, so we need to build an env var containing the command to run later
ENV COMMAND "/app/${cmd}"
CMD ${COMMAND}