package main

import (
	"bytes"
	"encoding/json"

	. "vaijunto/cmd/servidor/protocoloREST"
	. "vaijunto/shared"
)

func ReservaCommit(transacaoID string, passageiro string, servidores []bool) {
	//corpo com id da transacao e identificador do passageiro
	body, _ := json.Marshal(PacoteHttp{
		//aaaaah ta passando o mesmo 2 vezes, posso so tirar e acessar por Reserva...
		TransacaoID: transacaoID,
		Reserva: ReservaChave{
			ReservaID: transacaoID,
			Servidor:  cfg.ServidorID,
			User:      passageiro,
		},
	})

	for i, url := range cfg.GetUrls() {
		if servidores[i] {
			go cliente.Post(url+"/reserva/commit", "application/json", bytes.NewBuffer(body))
		}
	}
}
