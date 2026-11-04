package main

import (
	"bytes"
	"encoding/json"
	"log"

	. "vaijunto/cmd/servidor/protocoloREST"
)

func ReservaAbort(transacaoID string, servidores []bool) {
	body, _ := json.Marshal(PacoteHttp{TransacaoID: transacaoID})

	for i, url := range cfg.GetUrls() {
		if servidores[i] {
			log.Printf("Enviando abort da transação %s, para o servidor %d", transacaoID, i+1)
			go cliente.Post(url+"/reserva/abort", "application/json", bytes.NewBuffer(body))
		}
	}
}
