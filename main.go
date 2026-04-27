package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

func main() {

	http.HandleFunc("/codigos", func(w http.ResponseWriter, r *http.Request) {

		//pega valor do campo game
		gameNameUrl := r.URL.Query().Get("game")

		//verifica se o nome é de um dos suportados
		ValidGameconfig, validGameStatus := verifyGameName(gameNameUrl)

		//verifica status, se o jogo é válido
		if validGameStatus != true {
			fmt.Fprintf(w, "Jogo inválido.")
			return
		}

		//pega o código do jogo se o jogo for válido
		codigos, err := getCodes(ValidGameconfig.Game8url, ValidGameconfig.RedeemUrl)

		if len(codigos) == 0 {
			fmt.Println("Erro ao obter códigos")
			return
		}

		//verifica se teve erro.
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "Houve um erro ao buscar os códigos.")
			return
		}

		//fmt.Println("codes: ", codigos)

		//definições do json
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(codigos)

		//salva codigos no json
		saveToJson(gameNameUrl, codigos)
	})

	fmt.Println("Server rodando na porta 8080")

	http.ListenAndServe(":8080", nil)

}

func getCodes(gameLink string, redeemLink string) ([]GiftCode, error) {

	var resultado = []GiftCode{}

	//pega a resposta da requisição e erro se houver
	res, err := http.Get(gameLink)

	if err != nil {
		ReturnError(err)
		return nil, err
	}

	//trava o fechamento da url no final do main()
	defer res.Body.Close()

	doc, _ := goquery.NewDocumentFromReader(res.Body)

	//extração
	doc.Find("tbody").Each(func(i int, s *goquery.Selection) {
		s.Find(".a-clipboard__textInput").Each(func(j int, item *goquery.Selection) {
			code, _ := item.Attr("value")

			//se código mão for vazio coleta
			if code != "" {
				link := redeemLink + code
				item := GiftCode{Codigo: code, Link: link}
				resultado = append(resultado, item)
			}

		})
	})

	return resultado, nil
}

// funcao que salva um json com o nome do jogo extraido
func saveToJson(gameNameUrl string, dados []GiftCode) {
	bytes, err := json.Marshal(dados)
	if err != nil {
		ReturnError(err)
		return
	}

	res := os.WriteFile(gameNameUrl+".json", bytes, 0644)

	if res != nil {
		ReturnError(res)
		return
	}

}

func verifyGameName(gameName string) (LinkConfig, bool) {

	config, status := jogos[gameName]

	return config, status
}

func ReturnError(erro error) {
	fmt.Printf("[ ERRO ] [%s]: %s ", time.Now(), erro)
}
