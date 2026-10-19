FROM gitea.macrodata.internal/macrodata/kong-keycloak-auth@sha256:7e97ecb218942b5a08e2dd7d22b6080280da339196732cd5e091cff29cd8ed48

COPY --chown=kong:kong kong.yml /etc/kong/kong.yml

ENV KONG_DATABASE=off \
    KONG_DECLARATIVE_CONFIG=/etc/kong/kong.yml

RUN kong-keycloak-auth validate /etc/kong/kong.yml

RUN KONG_ADMIN_LISTEN=off KONG_PROXY_LISTEN=127.0.0.1:8000 kong start \
    && kong health \
    && kong stop