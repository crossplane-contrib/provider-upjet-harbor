# Digest-pinned so Renovate tracks rebuilds of the `nonroot` tag explicitly
# instead of floating on it silently.
FROM gcr.io/distroless/static-debian13:nonroot@sha256:8ce6dd87e9823c5b48dcfb2c8604714ccb351ce59607327bcc4957448da279b5

ARG TARGETOS
ARG TARGETARCH

ENV USER_ID=65532

USER ${USER_ID}
EXPOSE 8080

ADD "bin/${TARGETOS}_${TARGETARCH}/provider" /usr/local/bin/provider

ENTRYPOINT ["provider"]
