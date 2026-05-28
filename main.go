package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type GiftCode struct {
	Codigo string `json:"code"`
	Link   string `json:"link"`
}

type LinkConfig struct {
	Game8url  string
	RedeemUrl string
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

// Cache global com RWMutex para concorrência
var (
	cacheMutex sync.Mutex
	cacheJogos = make(map[string][]GiftCode)
)

func main() {
	// Otimizações de RAM
	runtime.GOMAXPROCS(2)
	debug.SetGCPercent(20)
	debug.FreeOSMemory()

	// Worker de atualização em background
	// Roda imediatamente ao iniciar, depois repete a cada 2 minutos
	go func() {
		fmt.Println("[Init] Gerando cache inicial...")
		atualizarTodosOsCodigos()

		ticker := time.NewTicker(2 * time.Minute)
		for range ticker.C {
			fmt.Println("[Worker] Atualizando códigos (2 min)...")
			atualizarTodosOsCodigos()
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

	fmt.Println("[Server] Listening on :8080")
	http.ListenAndServe(":8080", nil)
}

func atualizarTodosOsCodigos() {
	for nomeJogo, config := range jogos {
		fmt.Printf("[Scraper] Buscando códigos: %s...\n", nomeJogo)

		codigos, err := getCodes(config.Game8url, config.RedeemUrl)
		if err != nil {
			ReturnError(fmt.Errorf("falha ao atualizar %s: %v", nomeJogo, err))
			continue
		}

		if len(codigos) == 0 {
			fmt.Printf("[Aviso] Nenhum código encontrado para %s\n", nomeJogo)
			continue
		}

		// Trava de escrita para atualizar o map global
		cacheMutex.Lock()
		cacheJogos[nomeJogo] = codigos
		cacheMutex.Unlock()

		saveToJson(nomeJogo, codigos)
	}
	fmt.Println("[Scraper] Sincronização concluída.")
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

	// Faz a busca pelos seletores HTML (mantive a sua lógica original)
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
		ReturnError(err)
		return
	}

	err = os.WriteFile(gameNameUrl+".json", bytes, 0644)
	if err != nil {
		ReturnError(err)
	}
}

func verifyGameName(gameName string) (LinkConfig, bool) {
	config, status := jogos[gameName]
	return config, status
}

func ReturnError(erro error) {
	fmt.Printf("[Erro] [%s]: %v\n", time.Now().Format("2006-01-02 15:04:05"), erro)
}
