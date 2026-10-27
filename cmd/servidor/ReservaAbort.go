package main

import (
	"bytes"
	"encoding/json"
	"net/http"
)

func ReservaAbort(transacaoID string) {
	body, _ := json.Marshal(PacoteHttp{TransacaoID: transacaoID})

	for _, url := range participantes {
		go http.Post(url+"/reserva/abort", "application/json", bytes.NewBuffer(body))
	}
}
