package main

import (
	"bytes"
	"encoding/json"
	"net/http"

	. "vaijunto/shared"
)

func ReservaCommit(transacaoID string, passageiro string) {
	//corpo com id da transacao e identificador do passageiro
	body, _ := json.Marshal(PacoteHttp{
		//aaaaah ta passando o mesmo 2 vezes, posso so tirar e acessar por Reserva...
		TransacaoID: transacaoID,
		Reserva: ReservaChave{
			ReservaID: transacaoID,
			Servidor:  servidor,
			User:      passageiro,
		},
	})
	for _, url := range participantes {
		go http.Post(url+"/reserva/commit", "application/json", bytes.NewBuffer(body))
	}
}
