package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"

	. "vaijunto/cmd/servidor/protocoloREST"
	. "vaijunto/shared"
)

func ReservaCommit(transacaoID string, passageiro string, servidores []bool) {
	//corpo com id da transacao e identificador do passageiro
	body, _ := json.Marshal(PacoteHttp{
		TransacaoID: transacaoID,
		Reserva: ReservaChave{
			ReservaID: transacaoID,
			Servidor:  cfg.ServidorID,
			User:      passageiro,
		},
	})

	for i, url := range cfg.GetUrls() {
		if servidores[i] {
			go func(url string, servidorDestino int) {
				resp, err := cliente.Post(url+"/reserva/commit", "application/json", bytes.NewBuffer(body))
				if err != nil {
					log.Printf("[RESERVA COMMIT] Erro ao enviar commit para servidor %d (%s): %v", servidorDestino, url, err)
					return
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					log.Printf("[RESERVA COMMIT] Servidor %d retornou status %d para transação %s", servidorDestino, resp.StatusCode, transacaoID)
				}
			}(url, i+1)
		}
	}
}
