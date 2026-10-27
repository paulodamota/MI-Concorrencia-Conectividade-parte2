package main

import (
	"encoding/json"
	"net/http"
)

func handlePostReservaCommit(w http.ResponseWriter, r *http.Request) {
	//trata erro de leitura
	var req PacoteHttp
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	transacoes.mu.Lock()
	defer transacoes.mu.Unlock()

	trechosChaves, existe := transacoes.TrechosDaTransacao[req.TransacaoID]

	if !existe {
		http.Error(w, "Transação não encontrada", http.StatusNotFound)
		return
	}

	for _, tc := range trechosChaves {

		//acessa no mapa a viagem que contem o trecho
		ViagensMoto.mu.Lock()
		viagem, existe := ViagensMoto.Viagens[tc.UserName][tc.ViagemID]

		//se não existe cancela ??
		if !existe {
			ViagensMoto.mu.Unlock()
			http.Error(w, "Viagem não encontrada", http.StatusNotFound)
			return
		}

		ViagensMoto.mu.Unlock()

		//procura o trecho na viagem
		viagem.mu.Lock()
		for _, t := range viagem.Trechos {

			//decrementa vaga no trecho reservado
			if t.ID == tc.ID {
				t.Vagas--
				t.ReservasAssociadasIDS = append(t.ReservasAssociadasIDS, req.Reserva)
				break
			}
		}
		viagem.mu.Unlock()
	}

	for _, trechoChave := range trechosChaves {
		delete(transacoes.TransacaoDoTrecho, trechoChave)
	}
	delete(transacoes.TrechosDaTransacao, req.TransacaoID)

	//vota sim
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(PacoteHttp{Voto: true})
}
