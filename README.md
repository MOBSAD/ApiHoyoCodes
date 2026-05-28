# ApiHoyoCodes

  

Uma API de alta performance desenvolvida em Go para automação, scraping e consulta centralizada de códigos de resgate (Gift Codes) dos jogos da Hoyoverse (*Genshin Impact*, *Honkai: Star Rail* e *Zenless Zone Zero*).

  

O projeto evoluiu de um script simples de automação para um microserviço resiliente que opera com **processamento assíncrono em background**, **cache in-memory** thread-safe e **módulos de evasão anti-bot**.

  

## 📚 Evolução da Arquitetura

  

Este repositório documenta uma jornada de engenharia de software dividida em marcos de maturidade:

1. **O Nascimento (Scripting):** Criação de um scraper funcional básico usando `goquery`.

2. **Modularização:** Separação da lógica de extração da camada de transporte HTTP.

3. **Persistência Física:** Implementação de cache local em arquivos JSON para persistência fria.

4. **Alta Performance (Arquitetura Atual):** Transição para rotas REST puras não bloqueantes, onde os dados são servidos instantaneamente a partir da memória RAM protegida por travas concorrentes (`sync.Mutex`), enquanto um Worker em background atualiza o cache de forma independente.

  

## ⚙️ Tecnologias & Otimizações Internas
- **Go (Golang)** – Core runtime da aplicação.
- **Goquery** – Sintaxe fluída para análise e extração de dados DOM (HTML).
- **Runtime Fine-Tuning:** 
	-Limitação manual de concorrência com `runtime.GOMAXPROCS(2)`.
	-Controle agressivo de coleta de lixo com `debug.SetGCPercent(20)` para ambientes de baixo consumo (como setups embarcados ou mobile).

  

## 🚀 Funcionalidades Modernizadas
- **Rotas REST de Baixíssima Latência** 
	- - Respostas na casa dos microssegundos. O servidor consome o cache direto da memória RAM, eliminando o gargalo de IO ou requisições externas na rota do usuário.
- **Background Worker Autónomo** 
	- Um cronômetro interno (`time.Ticker`) varre as fontes de dados a cada **2 minutos** em uma Goroutine isolada, mantendo a API sempre atualizada sem congelar o servidor.
- **Evasão de Bloqueios Anti-Bot** 
	- Emissão de requisições HTTP forjando cabeçalhos de navegadores reais (*User-Agent* customizado) e mecanismos de *Timeout* estritos (10 segundos) para evitar travamentos por conexões zumbis.
- **Persistência Híbrida** 
	- Mantém cópias físicas atualizadas em arquivos `.json` estruturados de cada jogo como redundância local.

  

## 🛠️ Como Executar
### 1. Clone o repositório

```bash
git clone git@github.com:MOBSAD/ApiHoyoCodes.git
``` 
### 2.Instale as dependências

```bash
go mod tidy
``` 

### 3. Inicie o microserviço

``` Bash
go run main.go
```
### 4. Endpoints da API

- Acesse diretamente os recursos através de rotas limpas no navegador ou clientes HTTP (curl, Postman):

- **Genshin Impact**
  
```Plaintext
	http://localhost:3000/GI
```

- **Honkai: Star Rail**

```Plaintext
	http://localhost:3000/HSR
```

- **Zenless Zone Zero**

```Plaintext
	http://localhost:3000/ZZZ
```

### 🔐 Concorrência e Resiliência

A aplicação implementa primitivos primitivos de segurança em sistemas distribuídos:

1. **Exclusão Mútua** (`sync.Mutex`): Garante proteção total de leitura/escrita contra Data Races no mapa de memória durante os ciclos de varredura do scraper.
2. **Isolamento de Falhas**: Erros de rede nas requisições do scraper são capturados, logados com marca temporal pela função `ReturnError`, mas nunca derrubam o servidor HTTP principal.
