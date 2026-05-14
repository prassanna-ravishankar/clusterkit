FROM nginx:alpine

COPY web/ /usr/share/nginx/html/
COPY web/nginx.conf /etc/nginx/conf.d/default.conf
COPY web/security-headers.conf /etc/nginx/conf.d/security-headers.conf

RUN rm /usr/share/nginx/html/nginx.conf /usr/share/nginx/html/security-headers.conf

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost/health || exit 1

EXPOSE 80
