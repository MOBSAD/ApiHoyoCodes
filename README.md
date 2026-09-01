# ApiHoyoCodes

API em Go que busca códigos de Genshin Impact (GI), Honkai: Star Rail (HSR) e
Zenless Zone Zero (ZZZ).

O programa mantém uma API HTTP local com os códigos encontrados e tenta
resgatá-los automaticamente na HoYoverse. GI, HSR e ZZZ usam a mesma fila, com
um único worker processando os códigos em sequência e respeitando o intervalo
configurado entre cada tentativa.

Os códigos são coletados novamente a cada cinco minutos. Códigos já resgatados,
utilizados, expirados, inválidos ou com limite de uso atingido ficam salvos em um
histórico local e são ignorados nas próximas coletas. O terminal mostra esse
processo em um dashboard que é atualizado sem criar centenas de linhas.

## Como funciona

```text
Game8
  ↓
busca os códigos disponíveis de cada jogo
  ↓
atualiza o cache e os arquivos JSON locais
  ↓
verifica redeem_history.json
  ↓
ignora códigos já finalizados
  ↓
adiciona os restantes na redeemQueue
  ↓
redeemWorker processa um por vez
  ↓
HoYoverse
```

O scraper e o worker de resgate rodam em background. A fila evita códigos
duplicados enquanto eles estão aguardando ou sendo processados.

Existem dois intervalos diferentes:

- Coleta: o Game8 é consultado novamente cinco minutos depois de cada coleta.
- Redeem: `REDEEM_DELAY_SECONDS` controla a espera entre um código e outro.

## Instalação

O projeto usa Go 1.26.2, conforme declarado no `go.mod`.

```bash
git clone https://github.com/MOBSAD/ApiHoyoCodes.git
cd ApiHoyoCodes
go mod download
```

## Configuração

Crie o `.env` a partir do exemplo:

```bash
cp .env.example .env
```

Configurações disponíveis:

```dotenv
HOYOVERSE_COOKIE=

GENSHIN_UID=
GENSHIN_REGION=os_usa
GENSHIN_LANG=pt

HSR_UID=
HSR_REGION=
HSR_LANG=

ZZZ_UID=
ZZZ_REGION=
ZZZ_LANG=

REDEEM_DELAY_SECONDS=30
TEST_REDEEM_GAME=
TEST_REDEEM_CODE=
```

- `HOYOVERSE_COOKIE`: sessão válida da conta HoYoverse usada para autenticar os
  resgates dos três jogos.
- `GENSHIN_UID`, `HSR_UID` e `ZZZ_UID`: UID da conta em cada jogo.
- `GENSHIN_REGION`, `HSR_REGION` e `ZZZ_REGION`: região correspondente ao
  personagem de cada jogo.
- `GENSHIN_LANG`, `HSR_LANG` e `ZZZ_LANG`: idioma da resposta. Quando vazio, o
  programa usa `pt`.
- `REDEEM_DELAY_SECONDS`: intervalo entre tentativas de resgate. Quando vazio ou
  inválido, o padrão é 30 segundos.
- `TEST_REDEEM_GAME` e `TEST_REDEEM_CODE`: permitem colocar um código manual na
  mesma fila usada pelo scraper.

O cookie não deve ser compartilhado, colocado diretamente no código ou incluído
em exemplos reais. O `.env` já está no `.gitignore` e não deve ser enviado para
o Git.

Se o arquivo `.env` não existir, o programa ainda tenta usar variáveis normais
do ambiente.

## Executando

Use:

```bash
go run .
```

Esse comando compila todos os arquivos Go do package atual, incluindo `main.go`
e `terminal.go`.

Também é possível gerar o binário:

```bash
go build .
./ApiHoyoCodes
```

## Dashboard

Em um terminal Linux com suporte a ANSI, a saída fica parecida com esta:

```text
[Execução #1]
[Init] GI, HSR e ZZZ inicializados.

Acesse os códigos manualmente:
GI  -> http://localhost:3000/GI
HSR -> http://localhost:3000/HSR
ZZZ -> http://localhost:3000/ZZZ

Coleta concluída
GI : 3 resgatáveis | 5 usados/inválidos
HSR: 2 resgatáveis | 8 usados/inválidos
ZZZ: 1 resgatável | 4 usados/inválidos
Redeem: aguardando códigos
Próxima coleta em: 04:59
```

As linhas de coleta, contadores, redeem e contagem regressiva são atualizadas no
mesmo lugar. Em ambientes sem ANSI, o programa usa uma saída simples sem depender
das animações.

## API local

O servidor roda por padrão em:

```text
http://localhost:3000
```

Endpoints disponíveis:

- `GET /GI`
- `GET /HSR`
- `GET /ZZZ`

Exemplos:

```bash
curl http://localhost:3000/GI
curl http://localhost:3000/HSR
curl http://localhost:3000/ZZZ
```

Resposta fictícia de `GET /GI`, seguindo o formato atual da API:

```json
[
  {
    "code": "CODIGO_FICTICIO",
    "link": "https://genshin.hoyoverse.com/pt/gift?code=CODIGO_FICTICIO"
  }
]
```

Essas rotas servem somente para consultar os códigos coletados. Não é necessário
acessar `/GI`, `/HSR` ou `/ZZZ` para ativar o resgate automático. O scraper e o
worker começam sozinhos quando o programa é iniciado.

## Histórico de códigos

O arquivo `redeem_history.json` guarda resultados definitivos separados por jogo
e código.

Entram no histórico:

- Código resgatado com sucesso.
- Código já utilizado ou resgatado anteriormente.
- Código expirado.
- Código definitivamente inválido.
- Código com limite de uso atingido.

Timeout, erro de conexão, HTTP inesperado, rate limit, falha temporária da API ou
resposta desconhecida não finalizam o código. Nesses casos ele pode voltar para a
fila em uma coleta futura.

O histórico é carregado quando o programa inicia e salvo imediatamente após um
resultado definitivo. A identificação sempre considera jogo + código. Não é
necessário editar esse arquivo manualmente.

## Teste manual

Para enviar um código manual pela fila normal:

```dotenv
TEST_REDEEM_GAME=GI
TEST_REDEEM_CODE=CODIGO_AQUI
```

Os jogos aceitos são `GI`, `HSR` e `ZZZ`. Depois do teste, deixe as duas
variáveis vazias para não tentar o código novamente ao iniciar o programa.

## Arquivos gerados

Durante a execução, o projeto pode criar:

- `GI.json`, `HSR.json` e `ZZZ.json`: cópias dos códigos encontrados pelo
  scraper. São reconstruídas automaticamente.
- `redeem_history.json`: histórico local de códigos finalizados.
- `redeem_history.json.tmp`: arquivo temporário usado durante a gravação segura
  do histórico.

Esses arquivos são locais e ficam fora do Git.
