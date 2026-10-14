# syntax=docker/dockerfile:1.7
FROM nginx:1.28-alpine@sha256:a8b39bd9cf0f83869a2162827a0caf6137ddf759d50a171451b335cecc87d236

LABEL org.opencontainers.image.source="https://github.com/Eternity714/sub2api"
LABEL org.opencontainers.image.description="Gkotta Docsify documentation with independently mounted Markdown"

COPY deploy/docsify/nginx-main.conf /etc/nginx/nginx.conf
COPY deploy/docsify/nginx.conf /etc/nginx/conf.d/default.conf
COPY deploy/docsify/runtime/ /opt/docsify/runtime/
COPY frontend/docs/ /usr/share/nginx/html/docs/
# The source README describes maintenance and is never part of the public site.
RUN rm -f /usr/share/nginx/html/docs/README.md

USER 101:101
ENTRYPOINT ["nginx"]
CMD ["-g", "daemon off;"]

EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -q -T 4 -O /dev/null http://127.0.0.1:8080/health || exit 1
