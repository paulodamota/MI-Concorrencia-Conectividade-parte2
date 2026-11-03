package main

import (
	"log"
	"net/http"
	"time"

	. "vaijunto/cmd/servidor/protocoloREST"
	. "vaijunto/shared"
)

func ReservaCommit(transacaoID string, passageiro string, servidores []bool) bool {
	//corpo com id da transacao e identificador do passageiro
	pacote := PacoteHttp{
		TransacaoID: transacaoID,
		Reserva: ReservaChave{
			ReservaID: transacaoID,
			Servidor:  cfg.ServidorID,
			User:      passageiro,
		},
	}

	comitado := true

	for i, url := range cfg.GetUrls() {
		if !servidores[i] {
			continue
		}
		servidorDestino := i + 1

		// tenta até 3 vezes: redes instáveis podem causar falha transitória
		sucesso := false
		for tentativa := 1; tentativa <= 3; tentativa++ {
			status, _, err := postHTTP(url+"/reserva/commit", pacote)
			if err == nil && status == http.StatusOK {
				log.Printf("[RESERVA COMMIT] Reserva concluída: servidor %d confirmou transação %s (tentativa %d)", servidorDestino, transacaoID, tentativa)
				sucesso = true
				break
			}
			if err != nil {
				log.Printf("[RESERVA COMMIT] Tentativa %d — erro ao enviar commit para servidor %d: %v", tentativa, servidorDestino, err)
			} else {
				log.Printf("[RESERVA COMMIT] Tentativa %d — servidor %d retornou status %d para transação %s", tentativa, servidorDestino, status, transacaoID)
			}
			if tentativa < 3 {
				time.Sleep(500 * time.Millisecond)
			}
		}

		if !sucesso {
			log.Printf("[RESERVA COMMIT] ATENÇÃO: servidor %d não confirmou commit da transação %s após 3 tentativas — commit parcial!", servidorDestino, transacaoID)
			comitado = false
		}
	}

	return comitado
}
