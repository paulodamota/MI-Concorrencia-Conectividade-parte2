package main

import (
	"bytes"
	"encoding/json"
)

func ReservaAbort(transacaoID string) {
	body, _ := json.Marshal(PacoteHttp{TransacaoID: transacaoID})

	for _, url := range participantes {
		go cliente.Post(url+"/reserva/abort", "application/json", bytes.NewBuffer(body))
	}
}
