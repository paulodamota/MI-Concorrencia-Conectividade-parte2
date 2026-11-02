package main

import (
	"encoding/json"
	"log"
	"net/http"
	. "vaijunto/cmd/servidor/protocoloREST"
)

func handlePostReservaCommit(w http.ResponseWriter, r *http.Request) {
	//trata erro de leitura
	var req PacoteHttp
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	//confere se a transação expirou ou não
	trechosChaves, existe := transacoes.IniciarCommit(req.TransacaoID)
	if !existe {
		log.Printf("[COMMIT] Transação %s rejeitada: expirou por timeout ou não foi encontrada", req.TransacaoID)
		http.Error(w, "Transação não encontrada ou expirada", http.StatusNotFound)
		return
	}

	//liberara as vagas ao final do commit
	defer transacoes.Liberar(req.TransacaoID)

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

	//vota sim
	w.WriteHeader(http.StatusOK)
	log.Printf("Transação %s confirmada com sucesso", req.TransacaoID)
}
