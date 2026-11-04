package main

import (
	"log"
	"net/http"
	"sync"
	"time"

	. "vaijunto/cmd/servidor/protocoloREST"
	. "vaijunto/shared"
)

const (
	retryBackoffInicial = 500 * time.Millisecond
	retryBackoffMaximo  = 30 * time.Second
)

// evita duas goroutines de retry para a mesma transação.
var retriesAtivos = struct {
	mu  sync.Mutex
	ids map[string]bool
}{ids: make(map[string]bool)}

func algumVerdadeiro(flags []bool) bool {
	for _, f := range flags {
		if f {
			return true
		}
	}
	return false
}

// faz a primeira rodada de commits em paralelo
func ReservaCommit(transacaoID string, passageiro string, servidores []bool) (aceito bool, completo bool) {
	if !algumVerdadeiro(servidores) {
		log.Printf("[RESERVA COMMIT] Erro: nenhum servidor participante informado para transação %s", transacaoID)
		GravarWAL(EntradaWAL{Tipo: WALDone, TxID: transacaoID})
		return false, false
	}

	pendentes := enviarCommits(transacaoID, passageiro, servidores)

	if !algumVerdadeiro(pendentes) {
		GravarWAL(EntradaWAL{Tipo: WALDone, TxID: transacaoID})
		return true, true
	}

	log.Printf("[RESERVA COMMIT] tx %s: commit pendente em %v — retry persistente em segundo plano", transacaoID, pendentes)
	go retryPersistente(transacaoID, passageiro, pendentes, retryBackoffInicial)
	return true, false
}

// manda um commit a cada servidor marcado em alvos, em paralelo.
// Devolve quais servidores ainda estão pendentes
func enviarCommits(transacaoID string, passageiro string, alvos []bool) []bool {
	pacote := PacoteHttp{
		TransacaoID: transacaoID,
		Reserva: ReservaChave{
			ReservaID: transacaoID,
			Servidor:  cfg.ServidorID,
			User:      passageiro,
		},
	}

	restantes := make([]bool, len(alvos))
	var wg sync.WaitGroup

	for i, url := range cfg.GetUrls() {
		if i >= len(alvos) || !alvos[i] {
			continue
		}
		wg.Add(1)
		go func(i int, url string) {
			defer wg.Done()
			status, _, err := postHTTP(url+"/reserva/commit", pacote)

			switch {
			case err == nil && status == http.StatusOK:
				log.Printf("[RESERVA COMMIT] servidor %d confirmou tx %s", i+1, transacaoID)
				GravarWAL(EntradaWAL{Tipo: WALAck, TxID: transacaoID, Servidor: i + 1})

			case err == nil && status == http.StatusNotFound:
				log.Printf("[RESERVA COMMIT] ATENÇÃO: servidor %d não conhece a tx %s (expirou/abortou) — INCONSISTÊNCIA, desistindo deste servidor", i+1, transacaoID)
				GravarWAL(EntradaWAL{Tipo: WALAck, TxID: transacaoID, Servidor: i + 1})

			default:
				restantes[i] = true
				if err != nil {
					log.Printf("[RESERVA COMMIT] servidor %d indisponível para tx %s: %v", i+1, transacaoID, err)
				} else {
					log.Printf("[RESERVA COMMIT] servidor %d retornou status %d para tx %s", i+1, status, transacaoID)
				}
			}
		}(i, url)
	}
	wg.Wait()
	return restantes
}

// insiste no commit dos servidores pendentes até todos
// confirmarem.
func retryPersistente(transacaoID string, passageiro string, pendentes []bool, atrasoInicial time.Duration) {
	retriesAtivos.mu.Lock()
	if retriesAtivos.ids[transacaoID] {
		retriesAtivos.mu.Unlock()
		return
	}
	retriesAtivos.ids[transacaoID] = true
	retriesAtivos.mu.Unlock()

	defer func() {
		retriesAtivos.mu.Lock()
		delete(retriesAtivos.ids, transacaoID)
		retriesAtivos.mu.Unlock()
	}()

	backoff := retryBackoffInicial
	time.Sleep(atrasoInicial)

	for tentativa := 1; ; tentativa++ {
		pendentes = enviarCommits(transacaoID, passageiro, pendentes)

		if !algumVerdadeiro(pendentes) {
			GravarWAL(EntradaWAL{Tipo: WALDone, TxID: transacaoID})
			log.Printf("[RETRY COMMIT] tx %s concluída após %d rodada(s)", transacaoID, tentativa)
			return
		}

		log.Printf("[RETRY COMMIT] tx %s ainda pendente em %v (rodada %d) — nova tentativa em %s",
			transacaoID, pendentes, tentativa, backoff)
		time.Sleep(backoff)
		if backoff *= 2; backoff > retryBackoffMaximo {
			backoff = retryBackoffMaximo
		}
	}
}
