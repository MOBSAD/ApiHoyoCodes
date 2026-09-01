package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type GameStats struct {
	Resgataveis int
	Finalizados int
}

type TerminalState struct {
	mutex           sync.Mutex
	ansi            bool
	iniciado        bool
	execucao        int
	extracao        string
	redeem          string
	countdown       string
	redeemVersao    int
	estatisticas    map[string]GameStats
	linhasDinamicas int
}

func novoTerminal() *TerminalState {
	info, err := os.Stdout.Stat()
	ansi := err == nil && info.Mode()&os.ModeCharDevice != 0 && os.Getenv("TERM") != "" && os.Getenv("TERM") != "dumb"

	return &TerminalState{
		ansi:            ansi,
		execucao:        1,
		extracao:        "Aguardando coleta...",
		redeem:          "Redeem: aguardando códigos",
		countdown:       "Próxima coleta em: --:--",
		linhasDinamicas: 6,
		estatisticas: map[string]GameStats{
			"GI":  {},
			"HSR": {},
			"ZZZ": {},
		},
	}
}

func (t *TerminalState) iniciar() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	fmt.Printf("[Execução #%d]\n", t.execucao)
	fmt.Println("[Init] GI, HSR e ZZZ inicializados.")
	fmt.Println()
	fmt.Println("Acesse os códigos manualmente:")
	fmt.Println("GI  -> http://localhost:3000/GI")
	fmt.Println("HSR -> http://localhost:3000/HSR")
	fmt.Println("ZZZ -> http://localhost:3000/ZZZ")
	fmt.Println()

	t.iniciado = true
	if t.ansi {
		for _, linha := range t.linhasAtuais() {
			fmt.Println(linha)
		}
	}
}

func (t *TerminalState) atualizarExecucao(execucao int) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if !t.iniciado {
		t.execucao = execucao
		return
	}
	if t.execucao == execucao {
		return
	}
	t.execucao = execucao
	if !t.ansi {
		fmt.Printf("\n[Execução #%d]\n", execucao)
		return
	}

	linhasAteTitulo := t.linhasDinamicas + 8
	fmt.Printf("\033[%dA\r\033[2K[Execução #%d]\033[%dB\r", linhasAteTitulo, execucao, linhasAteTitulo)
}

func (t *TerminalState) mostrarExtracao(texto string, typing bool) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if !t.ansi || !typing {
		t.extracao = texto
		if t.ansi {
			t.renderDinamico()
		} else if t.iniciado {
			fmt.Println(texto)
		}
		return
	}

	t.extracao = ""
	for _, caractere := range texto {
		t.extracao += string(caractere)
		t.renderDinamico()
		time.Sleep(15 * time.Millisecond)
	}
}

func (t *TerminalState) atualizarEstatisticas(jogo string, resgataveis int, finalizados int) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.estatisticas[jogo] = GameStats{Resgataveis: resgataveis, Finalizados: finalizados}
	if t.ansi {
		t.renderDinamico()
	} else if t.iniciado {
		fmt.Printf("[Coleta] %s | ✓ %d resgatáveis | ✗ %d usados/inválidos\n", jogo, resgataveis, finalizados)
	}
}

func (t *TerminalState) finalizarCodigo(jogo string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	stats := t.estatisticas[jogo]
	if stats.Resgataveis > 0 {
		stats.Resgataveis--
	}
	stats.Finalizados++
	t.estatisticas[jogo] = stats

	if t.ansi {
		t.renderDinamico()
	}
}

func (t *TerminalState) mostrarRedeem(texto string, typing bool) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.redeemVersao++
	if !t.ansi || !typing {
		t.redeem = texto
		if t.ansi {
			t.renderDinamico()
		} else if t.iniciado {
			fmt.Println(texto)
		}
		return
	}

	t.redeem = ""
	for _, caractere := range texto {
		t.redeem += string(caractere)
		t.renderDinamico()
		time.Sleep(15 * time.Millisecond)
	}
}

func (t *TerminalState) mostrarResultado(texto string) {
	t.mutex.Lock()
	t.redeemVersao++
	versao := t.redeemVersao
	t.redeem = texto
	if t.ansi {
		t.renderDinamico()
	} else if t.iniciado {
		fmt.Println(texto)
	}
	t.mutex.Unlock()

	go func() {
		time.Sleep(3 * time.Second)
		t.mutex.Lock()
		defer t.mutex.Unlock()
		if versao != t.redeemVersao {
			return
		}
		t.redeem = ""
		if t.ansi {
			t.renderDinamico()
		}
	}()
}

func (t *TerminalState) atualizarCountdown(tempo time.Duration) {
	if tempo < 0 {
		tempo = 0
	}
	segundos := int(tempo.Seconds() + 0.999)
	texto := fmt.Sprintf("Próxima coleta em: %02d:%02d", segundos/60, segundos%60)

	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.countdown = texto
	if t.ansi {
		t.renderDinamico()
	}
}

func (t *TerminalState) mostrarAviso(texto string) {
	t.mutex.Lock()
	iniciado := t.iniciado
	t.mutex.Unlock()
	if !iniciado {
		fmt.Printf("[Aviso] %s\n", texto)
		return
	}
	t.mostrarResultado("[Aviso] " + texto)
}

func (t *TerminalState) mostrarErro(texto string) {
	t.mutex.Lock()
	iniciado := t.iniciado
	t.mutex.Unlock()
	if !iniciado {
		fmt.Printf("[Erro] %s\n", texto)
		return
	}
	t.mostrarResultado("[!] " + texto)
}

func (t *TerminalState) renderDinamico() {
	if !t.iniciado || !t.ansi {
		return
	}

	fmt.Printf("\033[%dA", t.linhasDinamicas)
	for _, linha := range t.linhasAtuais() {
		fmt.Printf("\r\033[2K%s\n", linha)
	}
}

func (t *TerminalState) linhasAtuais() []string {
	return []string{
		t.extracao,
		formatarJogo("GI", t.estatisticas["GI"]),
		formatarJogo("HSR", t.estatisticas["HSR"]),
		formatarJogo("ZZZ", t.estatisticas["ZZZ"]),
		t.redeem,
		t.countdown,
	}
}

func formatarJogo(jogo string, stats GameStats) string {
	espacos := strings.Repeat(" ", 3-len(jogo))
	return fmt.Sprintf("%s%s: %d resgatáveis | %d usados/inválidos", jogo, espacos, stats.Resgataveis, stats.Finalizados)
}
