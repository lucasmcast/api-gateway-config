FROM kong:3.9.4

COPY --chown=kong:kong kong.yml /etc/kong/kong.yml

ENV KONG_DATABASE=off \
    KONG_DECLARATIVE_CONFIG=/etc/kong/kong.yml

RUN kong config parse /etc/kong/kong.yml > /dev/null

RUN KONG_ADMIN_LISTEN=off KONG_PROXY_LISTEN=127.0.0.1:8000 kong start \
    && kong health \
    && kong stop