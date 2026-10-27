package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func handlePostReservaPrepare(w http.ResponseWriter, r *http.Request) {
	//trata erro de leitura
	var req PacoteHttp
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	//constroi as chaves de trechos
	trechosChaves := make([]TrechoChaves, len(req.Trechos))
	for i, trechoData := range req.Trechos {
		trechosChaves[i] = TrechoChaves{
			ID:       trechoData.ID,
			ViagemID: trechoData.ViagemID,
			UserName: trechoData.Motorista}
	}

	transacoes.mu.Lock()
	defer transacoes.mu.Unlock()

	for _, tc := range trechosChaves {

		transacao, ocupado := transacoes.TransacaoDoTrecho[tc]
		if ocupado {
			log.Printf("Trecho %s já está em uma transação (%s)", tc.ID, transacao)
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(PacoteHttp{Voto: false})
			return
		}

		//acessa no mapa a viagem que contem o trecho
		ViagensMoto.mu.Lock()
		viagem, existe := ViagensMoto.Viagens[tc.UserName][tc.ViagemID]

		//se não existe cancela
		if !existe {
			ViagensMoto.mu.Unlock()
			http.Error(w, "Viagem não encontrada", http.StatusNotFound)
			return
		}

		ViagensMoto.mu.Unlock()

		//procura o trecho na viagem
		viagem.mu.Lock()
		for _, t := range viagem.Trechos {

			//se não tem vaga cancela
			if t.ID == tc.ID && t.Vagas <= 0 {
				viagem.mu.Unlock()
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(PacoteHttp{Voto: false})
				return
			}
		}
		viagem.mu.Unlock()
	}

	//insere as chaves dos trechos como valor do ID da transacao no mapa
	transacoes.TrechosDaTransacao[req.TransacaoID] = trechosChaves

	for _, trechoChave := range trechosChaves {
		transacoes.TransacaoDoTrecho[trechoChave] = req.TransacaoID
	}

	//vota sim
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(PacoteHttp{Voto: true})
}
