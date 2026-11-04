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

	// WAL PARTICIPANTE: descarta o prepare — na próxima startup não tentaremos commitar
	GravarWALParticipante(EntradaWALParticipante{
		Tipo: "ABORTED",
		TxID: req.TransacaoID,
	})

	// se o abort chegou após o restart e a tx estava em pendentes, remove
	pendentesCommit.mu.Lock()
	delete(pendentesCommit.txs, req.TransacaoID)
	pendentesCommit.mu.Unlock()

	log.Printf("Transação %s abortada com sucesso", req.TransacaoID)
	w.WriteHeader(http.StatusOK)
}
