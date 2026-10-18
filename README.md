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
Hom publica duas rotas GET explicitas do ms-payment no host `api-hom.macrodata.dev`,
sem autenticacao, para validar conectividade. Prd permanece sem rotas.
Adicionar um arquivo por servico no ambiente desejado. Nomes de services e
routes devem ser unicos dentro de cada ambiente. Usar DNS de Services Kubernetes.
Configuracoes de plugins ficam no service ou route conforme o escopo desejado.
Nao versionar tokens, client secrets ou outras credenciais nesses arquivos.

## Rota de diagnostico em hom

O backend atual oferece endpoints de teste, nao operacoes de pagamento.
As rotas aceitam apenas o host de hom e metodo GET. Cada caminho publico tem
um mapeamento explicito no plugin OSS `request-transformer`, com `replace.uri`,
`strip_path: false` e `path_handling: v0`. O destino e diretamente o Service
`ms-payment.ms-payment-hom.svc.cluster.local:80`, sem usar o Ingress do servico.

- `ms-payment-root`: `~/payment$` transforma `/payment` em `/`.
- `ms-payment-hello`: `~/payment/hello$` transforma `/payment/hello` em `/hello`.
- `/payment/healthz`, `/payment/hello/extra` e caminhos nao cadastrados retornam 404 do Kong.
- Barras finais nao sao aliases: `/payment/` e `/payment/hello/` retornam 404.

O Kong ancora essas regexes no inicio; `$` exige o final do caminho.
Adicionar endpoints exige declarar novas rotas com seus metodos e URI destino.
Nao adicionar uma rota generica por prefixo: ela publicaria caminhos nao revisados.

Depois do merge na main, aguardar pipeline, sincronizacao e rollout de hom:

```bash
curl -i --max-time 20 https://api-hom.macrodata.dev/payment
curl -i --max-time 20 https://api-hom.macrodata.dev/payment/hello
curl -i --max-time 20 https://api-hom.macrodata.dev/payment/healthz
curl -i --max-time 20 https://api-hom.macrodata.dev/payment/hello/extra
```

Esperado: os dois primeiros com 200 e JSON do ms-payment; os demais com 404 do
Kong. Configuracoes DNS/rede e a imagem efetiva do backend precisam ser validadas
no cluster. Estas rotas sao publicas e nao devem carregar dados sensiveis. Definir o
contrato de pagamentos e autenticacao antes de publicar operacoes reais.
O Ingress direto existente do ms-payment nao foi removido: validar o gateway
antes de desativa-lo no repositorio ms-payment-gitops. Enquanto existir, ele
continua sendo uma entrada independente do Kong.

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
