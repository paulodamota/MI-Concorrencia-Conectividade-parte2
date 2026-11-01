package main

import (
	"encoding/json"
	"log"
	"net/http"
	. "vaijunto/cmd/servidor/protocoloREST"
)

func handlePostReservaAbort(w http.ResponseWriter, r *http.Request) {
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

	for _, trechoChave := range trechosChaves {
		delete(transacoes.TransacaoDoTrecho, trechoChave)
	}
	delete(transacoes.TrechosDaTransacao, req.TransacaoID)

	log.Printf("Transação %s abortada com sucesso", req.TransacaoID)
	w.WriteHeader(http.StatusOK)
}
