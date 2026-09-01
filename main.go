package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/joho/godotenv"
)

type GiftCode struct {
	Codigo string `json:"code"`
	Link   string `json:"link"`
}

type LinkConfig struct {
	Game8url  string
	RedeemUrl string
}

type RedeemItem struct {
	Jogo   string
	Codigo string
}

var jogos = map[string]LinkConfig{
	"GI": {
		Game8url:  "https://game8.co/games/Genshin-Impact/archives/304759",
		RedeemUrl: "https://genshin.hoyoverse.com/pt/gift?code=",
	},
	"HSR": {
		Game8url:  "https://game8.co/games/Honkai-Star-Rail/archives/410296",
		RedeemUrl: "https://hsr.hoyoverse.com/gift?code=",
	},
	"ZZZ": {
		Game8url:  "https://game8.co/games/Zenless-Zone-Zero/archives/435683",
		RedeemUrl: "https://zenless.hoyoverse.com/redemption?code=",
	},
}

// Cache global com Mutex para concorrência
var (
	cacheMutex sync.Mutex
	cacheJogos = make(map[string][]GiftCode)

	redeemQueue      = make(chan RedeemItem, 100)
	redeemQueueMutex sync.Mutex
	codigosNaFila    = make(map[string]bool)

	historicoMutex sync.Mutex
	historicoAtivo = true
	historico      = map[string]map[string]string{
		"GI":  {},
		"HSR": {},
		"ZZZ": {},
	}

	terminal = novoTerminal()
)

func main() {
	// Carrega o arquivo .env e mantém as variáveis normais como alternativa
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		terminal.mostrarAviso(fmt.Sprintf("Não foi possível carregar o .env: %v", err))
	}

	// Carrega códigos já finalizados
	carregarHistorico()

	redeemDelay := 30
	if valorDelay := strings.TrimSpace(os.Getenv("REDEEM_DELAY_SECONDS")); valorDelay != "" {
		valorConvertido, err := strconv.Atoi(valorDelay)
		if err != nil || valorConvertido <= 0 {
			terminal.mostrarAviso("REDEEM_DELAY_SECONDS inválido. Usando 30 segundos.")
		} else {
			redeemDelay = valorConvertido
		}
	}

	// Otimizações de RAM
	runtime.GOMAXPROCS(2)
	debug.SetGCPercent(20)
	debug.FreeOSMemory()

	terminal.iniciar()

	// Worker de resgate em background
	go redeemWorker(time.Duration(redeemDelay) * time.Second)

	// Envia o código de teste uma única vez pela mesma fila dos códigos encontrados
	if codigoTeste := strings.TrimSpace(os.Getenv("TEST_REDEEM_CODE")); codigoTeste != "" {
		jogoTeste := strings.ToUpper(strings.TrimSpace(os.Getenv("TEST_REDEEM_GAME")))
		if _, jogoValido := verifyGameName(jogoTeste); jogoValido {
			if adicionarNaFila(jogoTeste, codigoTeste) {
				terminal.mostrarResultado(fmt.Sprintf("Código de teste adicionado à fila para %s", jogoTeste))
			}
		} else {
			terminal.mostrarAviso("TEST_REDEEM_GAME inválido. Use GI, HSR ou ZZZ.")
		}
	}

	// Worker de atualização em background
	go func() {
		for execucao := 1; ; execucao++ {
			terminal.atualizarExecucao(execucao)
			if execucao == 1 {
				terminal.mostrarExtracao("Extraindo códigos...", true)
			} else {
				terminal.mostrarExtracao(fmt.Sprintf("EXTRAINDO NOVAMENTE - EXEC #%d", execucao), true)
			}
			atualizarTodosOsCodigos()
			terminal.mostrarExtracao("Coleta concluída", false)
			aguardarProximaColeta(5 * time.Minute)
		}
	}()

	// Rota principal
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		gameNameUrl := strings.ToUpper(strings.TrimPrefix(r.URL.Path, "/"))

		if _, validGameStatus := verifyGameName(gameNameUrl); !validGameStatus {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "Jogo inválido. Use /GI, /HSR ou /ZZZ")
			return
		}

		// Trava de leitura rápida
		cacheMutex.Lock()
		codigos, existe := cacheJogos[gameNameUrl]
		cacheMutex.Unlock()

		if !existe || len(codigos) == 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "Cache vazio ou carregando. Tente novamente em instantes.")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(codigos)
	})

	if err := http.ListenAndServe(":3000", nil); err != nil {
		terminal.mostrarErro(fmt.Sprintf("falha no servidor HTTP: %v", err))
	}
}

func atualizarTodosOsCodigos() {
	for nomeJogo, config := range jogos {
		codigos, err := getCodes(config.Game8url, config.RedeemUrl)
		if err != nil {
			terminal.mostrarErro(fmt.Sprintf("falha ao atualizar %s: %v", nomeJogo, err))
			continue
		}

		if len(codigos) == 0 {
			terminal.atualizarEstatisticas(nomeJogo, 0, 0)
			terminal.mostrarAviso(fmt.Sprintf("Nenhum código encontrado para %s", nomeJogo))
			continue
		}

		// Trava de escrita para atualizar o map global
		cacheMutex.Lock()
		cacheJogos[nomeJogo] = codigos
		cacheMutex.Unlock()

		saveToJson(nomeJogo, codigos)

		// Ignora códigos já finalizados
		for _, codigo := range codigos {
			if codigoFinalizado(nomeJogo, codigo.Codigo) {
				continue
			}

			adicionarNaFila(nomeJogo, codigo.Codigo)
		}

		atualizarEstatisticasColeta(nomeJogo, codigos)
	}
}

func aguardarProximaColeta(intervalo time.Duration) {
	proximaColeta := time.Now().Add(intervalo)
	for {
		restante := time.Until(proximaColeta)
		if restante <= 0 {
			terminal.atualizarCountdown(0)
			return
		}

		terminal.atualizarCountdown(restante)
		time.Sleep(time.Second)
	}
}

func atualizarEstatisticasColeta(jogo string, codigos []GiftCode) {
	historicoMutex.Lock()
	defer historicoMutex.Unlock()

	resgataveis := 0
	finalizados := 0
	codigosContados := make(map[string]bool)
	for _, codigo := range codigos {
		if codigosContados[codigo.Codigo] {
			continue
		}
		codigosContados[codigo.Codigo] = true

		if historicoAtivo {
			if _, existe := historico[jogo][codigo.Codigo]; existe {
				finalizados++
				continue
			}
		}
		resgataveis++
	}

	terminal.atualizarEstatisticas(jogo, resgataveis, finalizados)
}

func codigoNaColetaAtual(jogo string, codigo string) bool {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	for _, codigoAtual := range cacheJogos[jogo] {
		if codigoAtual.Codigo == codigo {
			return true
		}
	}
	return false
}

func adicionarNaFila(jogo string, codigo string) bool {
	if codigoFinalizado(jogo, codigo) {
		return false
	}

	// Evita duplicados na fila
	chaveCodigo := jogo + ":" + codigo

	redeemQueueMutex.Lock()
	if codigosNaFila[chaveCodigo] {
		redeemQueueMutex.Unlock()
		return false
	}
	codigosNaFila[chaveCodigo] = true
	redeemQueueMutex.Unlock()

	redeemQueue <- RedeemItem{Jogo: jogo, Codigo: codigo}
	return true
}

func redeemWorker(redeemDelay time.Duration) {
	primeiroCodigo := true

	for codigo := range redeemQueue {
		// Aguarda antes dos próximos códigos, mas processa o primeiro imediatamente
		if !primeiroCodigo {
			time.Sleep(redeemDelay)
		}
		primeiroCodigo = false

		terminal.mostrarRedeem(fmt.Sprintf("Resgatando %s / %s...", codigo.Jogo, codigo.Codigo), true)

		var estado string
		var err error
		switch codigo.Jogo {
		case "GI":
			estado, err = redeemGenshinCode(codigo.Codigo)
		case "HSR":
			estado, err = redeemHSRCode(codigo.Codigo)
		case "ZZZ":
			estado, err = redeemZZZCode(codigo.Codigo)
		default:
			err = fmt.Errorf("jogo inválido: %s", codigo.Jogo)
		}

		if err != nil {
			terminal.mostrarResultado(fmt.Sprintf("! %s / %s erro temporário: %v", codigo.Jogo, codigo.Codigo, err))
		} else {
			if err := finalizarCodigo(codigo.Jogo, codigo.Codigo, estado); err != nil {
				terminal.mostrarErro(fmt.Sprintf("falha ao salvar histórico: %v", err))
			} else {
				switch estado {
				case "redeemed":
					terminal.mostrarResultado(fmt.Sprintf("✓ %s / %s resgatado", codigo.Jogo, codigo.Codigo))
				case "already_redeemed":
					terminal.mostrarResultado(fmt.Sprintf("✗ %s / %s já utilizado", codigo.Jogo, codigo.Codigo))
				case "expired":
					terminal.mostrarResultado(fmt.Sprintf("✗ %s / %s expirado", codigo.Jogo, codigo.Codigo))
				case "invalid":
					terminal.mostrarResultado(fmt.Sprintf("✗ %s / %s inválido", codigo.Jogo, codigo.Codigo))
				case "usage_limit":
					terminal.mostrarResultado(fmt.Sprintf("✗ %s / %s limite de uso atingido", codigo.Jogo, codigo.Codigo))
				}
			}
		}

		chaveCodigo := codigo.Jogo + ":" + codigo.Codigo
		redeemQueueMutex.Lock()
		delete(codigosNaFila, chaveCodigo)
		redeemQueueMutex.Unlock()
	}
}

func redeemGenshinCode(codigo string) (string, error) {
	return redeemCode(
		codigo,
		"GI",
		"https://public-operation-hk4e.hoyoverse.com/common/apicdkey/api/webExchangeCdkey",
		"hk4e_global",
		"GENSHIN_UID",
		"GENSHIN_REGION",
		"GENSHIN_LANG",
		"https://genshin.hoyoverse.com/pt/gift",
	)
}

func redeemHSRCode(codigo string) (string, error) {
	return redeemCode(
		codigo,
		"HSR",
		"https://public-operation-hkrpg.hoyoverse.com/common/apicdkey/api/webExchangeCdkey",
		"hkrpg_global",
		"HSR_UID",
		"HSR_REGION",
		"HSR_LANG",
		"https://hsr.hoyoverse.com/gift",
	)
}

func redeemZZZCode(codigo string) (string, error) {
	return redeemCode(
		codigo,
		"ZZZ",
		"https://public-operation-nap.hoyoverse.com/common/apicdkey/api/webExchangeCdkey",
		"nap_global",
		"ZZZ_UID",
		"ZZZ_REGION",
		"ZZZ_LANG",
		"https://zenless.hoyoverse.com/redemption",
	)
}

func redeemCode(codigo string, jogo string, endpoint string, gameBiz string, uidEnv string, regionEnv string, langEnv string, referer string) (string, error) {
	cookie := os.Getenv("HOYOVERSE_COOKIE")
	uid := os.Getenv(uidEnv)
	region := os.Getenv(regionEnv)
	lang := os.Getenv(langEnv)

	if cookie == "" || uid == "" || region == "" {
		return "", fmt.Errorf("configure HOYOVERSE_COOKIE, %s e %s para %s", uidEnv, regionEnv, jogo)
	}

	if lang == "" {
		lang = "pt"
	}

	// Monta a mesma requisição usada pelas páginas oficiais de resgate da HoYoverse
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", err
	}

	params := req.URL.Query()
	params.Set("uid", uid)
	params.Set("region", region)
	params.Set("lang", lang)
	params.Set("cdkey", codigo)
	params.Set("game_biz", gameBiz)
	req.URL.RawQuery = params.Encode()

	req.Header.Set("Cookie", cookie)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", referer)

	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HoYoverse retornou status: %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	var resultado struct {
		Retcode int    `json:"retcode"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &resultado); err != nil {
		return "", err
	}

	if estado, definitivo := classificarResultado(resultado.Retcode, resultado.Message); definitivo {
		return estado, nil
	}

	return "", fmt.Errorf("HoYoverse retornou %d: %s", resultado.Retcode, resultado.Message)
}

func classificarResultado(retcode int, mensagem string) (string, bool) {
	if retcode == 0 {
		return "redeemed", true
	}

	switch retcode {
	case -2001:
		return "invalid", true
	case -2003:
		return "expired", true
	case -2016:
		return "usage_limit", true
	case -2017:
		return "already_redeemed", true
	}

	mensagem = strings.ToLower(strings.TrimSpace(mensagem))
	if contemAlgum(mensagem,
		"already redeemed", "already claimed", "already in use", "already used",
		"já utilizado", "ja utilizado", "já resgatado", "ja resgatado",
		"já foi usado", "ja foi usado") {
		return "already_redeemed", true
	}
	if contemAlgum(mensagem, "expired", "expirado", "expirada") {
		return "expired", true
	}
	if contemAlgum(mensagem,
		"invalid redemption code", "redemption code is invalid", "invalid code",
		"código de resgate inválido", "codigo de resgate invalido",
		"código inválido", "codigo invalido") {
		return "invalid", true
	}
	if contemAlgum(mensagem,
		"usage limit", "maximum number of uses", "redemption limit reached",
		"limite de uso", "limite de resgates") {
		return "usage_limit", true
	}

	return "", false
}

func contemAlgum(texto string, termos ...string) bool {
	for _, termo := range termos {
		if strings.Contains(texto, termo) {
			return true
		}
	}
	return false
}

func carregarHistorico() {
	bytes, err := os.ReadFile("redeem_history.json")
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		terminal.mostrarAviso(fmt.Sprintf("Não foi possível carregar redeem_history.json: %v", err))
		historicoAtivo = false
		return
	}
	if len(strings.TrimSpace(string(bytes))) == 0 {
		return
	}

	var dados map[string]map[string]string
	if err := json.Unmarshal(bytes, &dados); err != nil {
		terminal.mostrarAviso(fmt.Sprintf("redeem_history.json contém JSON inválido e não será sobrescrito: %v", err))
		historicoAtivo = false
		return
	}

	for nomeJogo := range jogos {
		if dados[nomeJogo] == nil {
			dados[nomeJogo] = make(map[string]string)
		}
	}
	historico = dados
}

func codigoFinalizado(jogo string, codigo string) bool {
	historicoMutex.Lock()
	defer historicoMutex.Unlock()

	if !historicoAtivo {
		return false
	}

	_, existe := historico[jogo][codigo]
	return existe
}

func finalizarCodigo(jogo string, codigo string, estado string) error {
	historicoMutex.Lock()
	defer historicoMutex.Unlock()

	if !historicoAtivo {
		return fmt.Errorf("histórico desativado para proteger o arquivo atual")
	}
	if historico[jogo] == nil {
		historico[jogo] = make(map[string]string)
	}
	estadoAnterior, existia := historico[jogo][codigo]
	historico[jogo][codigo] = estado
	desfazer := func() {
		if existia {
			historico[jogo][codigo] = estadoAnterior
		} else {
			delete(historico[jogo], codigo)
		}
	}

	// Salva histórico local
	bytes, err := json.MarshalIndent(historico, "", "  ")
	if err != nil {
		desfazer()
		return err
	}
	if err := os.WriteFile("redeem_history.json.tmp", bytes, 0644); err != nil {
		desfazer()
		return err
	}
	if err := os.Rename("redeem_history.json.tmp", "redeem_history.json"); err != nil {
		desfazer()
		return err
	}

	if codigoNaColetaAtual(jogo, codigo) {
		terminal.finalizarCodigo(jogo)
	}

	return nil
}

func getCodes(gameLink string, redeemLink string) ([]GiftCode, error) {
	var resultado []GiftCode

	// Cria a requisição manualmente em vez de usar http.Get direto
	req, err := http.NewRequest("GET", gameLink, nil)
	if err != nil {
		return nil, err
	}

	// Adiciona um User-Agent fingindo ser o Google Chrome no Windows
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")

	// Usa um Client com timeout de 10 segundos para o scraper não ficar travado infinitamente
	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("site retornou status: %d", res.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, err
	}

	// Faz a busca pelos seletores HTML
	doc.Find("tbody").Each(func(i int, s *goquery.Selection) {
		s.Find(".a-clipboard__textInput").Each(func(j int, item *goquery.Selection) {
			code, _ := item.Attr("value")

			if code != "" {
				resultado = append(resultado, GiftCode{
					Codigo: code,
					Link:   redeemLink + code,
				})
			}
		})
	})

	return resultado, nil
}

func saveToJson(gameNameUrl string, dados []GiftCode) {
	bytes, err := json.MarshalIndent(dados, "", "  ")
	if err != nil {
		terminal.mostrarErro(err.Error())
		return
	}

	err = os.WriteFile(gameNameUrl+".json", bytes, 0644)
	if err != nil {
		terminal.mostrarErro(err.Error())
	}
}

func verifyGameName(gameName string) (LinkConfig, bool) {
	config, status := jogos[gameName]
	return config, status
}
