package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"

	. "vaijunto/cmd/servidor/protocoloREST"
	. "vaijunto/shared"
)

func ReservaCommit(transacaoID string, passageiro string, servidores []bool) bool {
	//corpo com id da transacao e identificador do passageiro
	body, _ := json.Marshal(PacoteHttp{
		TransacaoID: transacaoID,
		Reserva: ReservaChave{
			ReservaID: transacaoID,
			Servidor:  cfg.ServidorID,
			User:      passageiro,
		},
	})

	var comitado bool

	for i, url := range cfg.GetUrls() {
		if servidores[i] {
			servidorDestino := i + 1

			resp, err := cliente.Post(url+"/reserva/commit", "application/json", bytes.NewBuffer(body))
			if err != nil {
				log.Printf("[RESERVA COMMIT] Erro ao enviar commit para servidor %d (%s): %v", servidorDestino, url, err)
				comitado = false
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				log.Printf("[RESERVA COMMIT] Servidor %d retornou status %d para transação %s", servidorDestino, resp.StatusCode, transacaoID)
				comitado = false
			} else {
				log.Printf("[RESERVA COMMIT] Reserva concluida: Servidor %d retornou status %d para transação %s", servidorDestino, resp.StatusCode, transacaoID)
				comitado = true
			}

		}
	}
	return comitado
}
