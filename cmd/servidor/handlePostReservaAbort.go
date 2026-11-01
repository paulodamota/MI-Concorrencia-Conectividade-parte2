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

	transacoes.Liberar(req.TransacaoID)

	log.Printf("Transação %s abortada com sucesso", req.TransacaoID)
	w.WriteHeader(http.StatusOK)
}
