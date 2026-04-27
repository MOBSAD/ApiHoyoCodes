# ApiHoyoCodes

Uma API robusta desenvolvida em Go para automação e consulta de códigos de resgate (Gift Codes) dos jogos da Hoyoverse (Genshin Impact, Honkai: Star Rail e Zenless Zone Zero).

O projeto nasceu de um simples web scraper e evoluiu para um microserviço capaz de gerenciar múltiplos jogos, realizar persistência em cache local e tratar erros de forma resiliente.

## Evolução do Projeto

Este repositório documenta uma jornada de aprendizado dividida em capítulos:
1. **O Nascimento**: Criação de um scraper funcional usando `goquery`.
2. **Modularização**: Separação da lógica de busca da lógica do servidor.
3. **Persistência**: Implementação de cache local em JSON para evitar requisições desnecessárias.
4. **Escalabilidade**: Uso de `structs` de configuração para suportar novos jogos dinamicamente.
5. **Segurança**: Implementação de tratamento de erros e proteção contra vazamento de chaves no Git.

## Tecnologias Utilizadas

- **Go (Golang)**: Linguagem principal.
- **Goquery**: Para análise e extração de dados HTML.
- **Standard Library (`net/http`, `encoding/json`, `os`)**: Para criação da API e manipulação de arquivos.

## Funcionalidades

- **Busca Dinâmica**: Consulta códigos de diferentes jogos via parâmetros de URL.
- **Geração de Links de Resgate**: Constrói automaticamente o link oficial para resgate direto.
- **Cache Inteligente**: Salva os códigos encontrados em arquivos `.json` nomeados por jogo.
- **Tratamento de Erros**: Sistema de log centralizado e prevenção de quedas do servidor.

## Como Executar

1. **Clone o repositório:**
  ```bash
   git clone git@github.com:MOBSAD/ApiHoyoCodes.git```

2. **Instale as dependências:**
  ```bash
    go mod tidy```

3. **Inicie o servidor:**
  ```bash
    go run main.go```

4. **Acesse via navegador ou Insomnia/Postman:**

- **Para Genshin Impact**: http://localhost:8080/codigos?game=GI
- **Para Honkai Star Rail**: http://localhost:8080/codigos?game=HSR
- **Para Zenless Zone Zero**: http://localhost:8080/codigos?game=ZZZ

## Segurança

- Este projeto implementa boas práticas de segurança, incluindo:
1. Verificação de nomes de jogos suportados antes de processar requisições.
2. Prevenção de log.Fatal em rotas críticas para manter o servidor online.
