package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	. "vaijunto/cmd/servidor/protocoloREST"
)

// commitadas guarda os IDs das transações já aplicadas com sucesso nesta execução.
// Garante idempotência: se o coordenador reenviar o commit (ex: recovery WAL ou retry de rede),
// não decrementamos Vagas duas vezes.
var commitadas = struct {
	mu  sync.Mutex
	ids map[string]bool
}{ids: make(map[string]bool)}

func handlePostReservaCommit(w http.ResponseWriter, r *http.Request) {
	//trata erro de leitura
	var req PacoteHttp
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// idempotência: se já foi commitada nesta execução, retorna 200 sem reprocessar
	commitadas.mu.Lock()
	jaAplicada := commitadas.ids[req.TransacaoID]
	commitadas.mu.Unlock()

	if jaAplicada {
		log.Printf("[COMMIT] Transação %s já foi aplicada anteriormente (idempotente — ignorando reenvio)", req.TransacaoID)
		w.WriteHeader(http.StatusOK)
		return
	}

	// tenta obter as chaves pela rota normal (trava ainda em memória)
	trechosChaves, existe := transacoes.IniciarCommit(req.TransacaoID)

	if !existe {
		// rota de recovery: participante reiniciou e a trava expirou,
		// mas o WAL registrou o prepare — podemos aplicar o commit assim mesmo
		pendentesCommit.mu.Lock()
		chavesPendentes, emPendentes := pendentesCommit.txs[req.TransacaoID]
		if emPendentes {
			delete(pendentesCommit.txs, req.TransacaoID)
		}
		pendentesCommit.mu.Unlock()

		if emPendentes {
			log.Printf("[COMMIT] Tx %s aplicada via recovery do WAL do participante", req.TransacaoID)
			trechosChaves = chavesPendentes
			// continua com o commit abaixo (sem defer Liberar — lock já não existe)
			aplicarCommitTrechos(w, req, trechosChaves)
			return
		}

		// tx genuinamente desconhecida: nunca foi preparada ou expirou sem registro
		log.Printf("[COMMIT] Transação %s rejeitada: expirou por timeout ou não foi encontrada", req.TransacaoID)
		http.Error(w, "Transação não encontrada ou expirada", http.StatusNotFound)
		return
	}

	// rota normal: trava ainda estava em memória — libera ao final
	defer transacoes.Liberar(req.TransacaoID)
	aplicarCommitTrechos(w, req, trechosChaves)
}

// aplicarCommitTrechos decrementa Vagas nos trechos e responde 200.
// Separado para ser usado tanto pela rota normal quanto pelo recovery.
func aplicarCommitTrechos(w http.ResponseWriter, req PacoteHttp, trechosChaves []TrechoChaves) {
	for _, tc := range trechosChaves {

		//acessa no mapa a viagem que contem o trecho
		ViagensMoto.mu.Lock()
		viagem, existe := ViagensMoto.Viagens[tc.UserName][tc.ViagemID]
		ViagensMoto.mu.Unlock()

		//se não existe cancela ??
		if !existe {
			http.Error(w, "Viagem não encontrada", http.StatusNotFound)
			return
		}

		//procura o trecho na viagem
		viagem.mu.Lock()
		for _, t := range viagem.Trechos {

			//decrementa vaga no trecho reservado
			if t.ID == tc.ID {
				t.mu.Lock()
				t.Vagas--
				t.ReservasAssociadasIDS = append(t.ReservasAssociadasIDS, req.Reserva)
				t.mu.Unlock()
				break
			}
		}
		viagem.mu.Unlock()
	}

	// marca como commitada nesta execução para garantir idempotência em reenvios
	commitadas.mu.Lock()
	commitadas.ids[req.TransacaoID] = true
	commitadas.mu.Unlock()

	// registra no WAL do participante que o commit foi aplicado
	GravarWALParticipante(EntradaWALParticipante{
		Tipo: "COMMITTED",
		TxID: req.TransacaoID,
	})

	w.WriteHeader(http.StatusOK)
	log.Printf("Transação %s confirmada com sucesso", req.TransacaoID)
}
