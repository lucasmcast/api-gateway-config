# API Gateway Config

Configuracao declarativa dos gateways Kong OSS DB-less. O repositorio
`platform-gitops` mantem a infraestrutura; este fornece rotas e o Dockerfile.
Nao cria recursos Kubernetes: a pipeline publica imagens e atualiza o GitOps.

## Estrutura

```text
gateways/public-api/hom/services/ms-payment.yaml
gateways/public-api/prd/services/ms-payment.yaml
Dockerfile
cmd/gateway/
internal/gateway/
```

Cada arquivo de entrada contem `services: [...]` no formato declarativo do Kong.
Os arquivos iniciais estao vazios: nenhuma API ou autenticacao e publicada.
Adicionar um arquivo por servico no ambiente desejado. Nomes de services e
routes devem ser unicos dentro de cada ambiente. Usar DNS de Services Kubernetes.
Configuracoes de plugins ficam no service ou route conforme o escopo desejado.
Nao versionar tokens, client secrets ou outras credenciais nesses arquivos.

## Publicacao por imagem

A pipeline exclusiva `kong-publish` do `tekton-catalog` recebe pushes na main.
Cada evento cria uma execucao por ambiente; somente constroi e publica quando
o fingerprint de Dockerfile e kong.yml difere da ultima publicacao.
Na primeira execucao ambos os ambientes sao publicados.

Fluxo: merge -> testes -> contexto temporario -> build e push -> atualizacao
dos valores por ambiente no platform-gitops -> sincronizacao automatica Argo CD.
Versionar apenas as entradas, o Dockerfile, o codigo Go e suas dependencias.
O YAML consolidado existe somente no contexto temporario de build.

Para testar o fluxo de imagem localmente:

```bash
go mod download
go test ./...
go run ./cmd/gateway image-context hom
docker build -t kong-routes:local dist/hom
```

O contexto `dist/hom` e temporario e ignorado pelo Git. O Dockerfile valida
o schema e testa startup/health/stop com o Kong real antes de publicar.
Adicionar instalacao e habilitacao de plugins no Dockerfile antes de usa-los.
Hoje o contexto contem apenas Dockerfile e kong.yml: plugins devem ser instalados
por dependencias fixadas no Dockerfile; copiar plugins locais exigira ampliar
o gerador de contexto e o fingerprint. Nao embutir credenciais na imagem.

A pipeline grava repository e digest em `platforms/kong/environments/<env>.yaml`.
O Argo CD acompanha esses arquivos; o bootstrap so precisa ser aplicado uma vez
para passar a referencia-los. Nao precisa reaplica-lo por publicacao.

Configurar webhook, permissoes e registry conforme `docs/kong-publish.md` no
tekton-catalog. Publicacao termina no push GitOps: nao aguarda health do Argo CD
nem testa backends. Acompanhar rollout e dominios apos publicar. Para rollback,
reverter a referencia em platform-gitops; se quiser manter o rollback, reverter
tambem as entradas que causaram a mudanca antes do proximo push de rotas.

## Validacao

Requer Go 1.25 ou superior. O CLI usa go.yaml.in/yaml/v3 para parsing estruturado
e preserva campos de services, routes e plugins. Executar na raiz do repositorio.
Go e usado apenas na preparacao, nao na imagem final do Kong.

Para validar os manifests Tekton:

```bash
go run ./cmd/gateway validate-tekton ../tekton-catalog
```

O gerador faz validacoes estruturais basicas; nao valida todos os conflitos
semanticos ou a seguranca dos plugins. O build valida com a versao real do Kong.
Proteger main e exigir revisao para prd: alteracoes aprovadas sao publicadas
automaticamente. Nao publicar rotas sensiveis antes de escolher e validar
autenticacao. A presenca de um plugin qualquer nao garante protecao.
