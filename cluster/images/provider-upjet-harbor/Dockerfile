# Digest-pinned so Renovate tracks rebuilds of the `nonroot` tag explicitly
# instead of floating on it silently.
FROM gcr.io/distroless/static-debian13:nonroot@sha256:516757106de35205efb70b91ba03f6f1298424a0678c603255cf2852a7000087

ARG TARGETOS
ARG TARGETARCH

ENV USER_ID=65532

USER ${USER_ID}
EXPOSE 8080

ADD "bin/${TARGETOS}_${TARGETARCH}/provider" /usr/local/bin/provider

ENTRYPOINT ["provider"]
