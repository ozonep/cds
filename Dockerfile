FROM newcds:builder5 as sourcer

FROM alpine:3.12
RUN apk upgrade -U -a && apk add --no-cache bash curl tar ca-certificates busybox-extras bind-tools git gnupg && \
    mkdir -p /app/sql /app/ui_static_files /app/panic_dumps
COPY --from=sourcer app/engine/dist/cds-engine-* /app/
COPY --from=sourcer app/cli/cdsctl/dist/cdsctl-* /app/
COPY --from=sourcer app/engine/worker/dist/cds-worker-* /app/
COPY --from=sourcer app/engine/dist/sql.tar.gz /app/
COPY --from=sourcer app/ui/ui.tar.gz /app/
# COPY docpublic/. /app/doc_static_files/docs/
# COPY /configdev.toml /app/conf/conf.toml
# RUN groupadd -r cds && useradd --create-home -r -g cds cds
RUN addgroup -g 1000 -S cds && adduser -u 1000 -S cds -G cds -s /bin/bash
RUN chmod +w /app/panic_dumps && \
    chmod +x /app/cds-engine-linux-amd64 && \
    tar xzf /app/sql.tar.gz -C /app/sql && \
    tar xzf /app/ui.tar.gz -C /app/ui_static_files && \
    rm /app/ui.tar.gz && rm /app/sql.tar.gz && rm /app/cdsctl-linux-amd64 && \
    find /app -type f \( -name '*.eot' -o -name '*.ttf' -o -name '*.svg' -o -name '*.woff' \) -print -delete && \
    sed -i '/.eot;/d' /app/ui_static_files/dist/FILES_UI && \
    sed -i '/.svg;/d' /app/ui_static_files/dist/FILES_UI && \
    sed -i '/.woff;/d' /app/ui_static_files/dist/FILES_UI && \
    sed -i '/.txt;/d' /app/ui_static_files/dist/FILES_UI && \
    sed -i '/.ttf;/d' /app/ui_static_files/dist/FILES_UI && \
    rm /app/ui_static_files/dist/3rdpartylicenses.txt && \
    # mv /app/doc_static_files/docs /app/ui_static_files/docs && \
    chown -R cds:cds /app
USER cds
WORKDIR /app
CMD ["/app/cds-engine-linux-amd64"]