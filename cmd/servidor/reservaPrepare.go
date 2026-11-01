package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/google/uuid"

	. "vaijunto/shared"
)

// tenta reservar vagas em trechos
func ReservaPrepare(viagemData *ViagemData, ts Timestamp) (string, bool) {

	log.Printf("[RESERVA PREPARE] 1")

	if len(viagemData.Trechos) == 0 {
		return "", false
	}

	transacaoID := uuid.New().String()

	trechos := make([][]TrechoData, 3)

	for _, trechoData := range viagemData.Trechos {
		switch trechoData.Servidor {
		case 1:
			trechos[0] = append(trechos[0], trechoData)
			log.Printf("[RESERVA PREPARE] 2, requisicao para servidor 1")
		case 2:
			trechos[1] = append(trechos[1], trechoData)
			log.Printf("[RESERVA PREPARE] 3, requisicao para servidor 2")
		case 3:
			trechos[2] = append(trechos[2], trechoData)
			log.Printf("[RESERVA PREPARE] 4, requisicao para servidor 3")
		}
	}

	requisicoes := 0
	for _, TrechosDoServidorI := range trechos {
		if len(TrechosDoServidorI) > 0 {
			requisicoes++
		}
	}

	log.Printf("[RESERVA PREPARE] 5, requisicoes: %d", requisicoes)

	var wg sync.WaitGroup
	log.Printf("[RESERVA PREPARE] 6")

	//++[ ENVIO DE PREPARE ]++
	votos := make(chan bool, requisicoes)
	log.Printf("[RESERVA PREPARE] 7")

	for i, url := range cfg.GetUrls() {
		log.Printf("[RESERVA PREPARE] 8.%d", i)

		var req PacoteHttp
		req.TransacaoID = transacaoID
		req.Timestamp = ts

		//construindo requisicao com os trechos desejados
		switch i {
		case 0:
			req.Trechos = trechos[0]
		case 1:
			req.Trechos = trechos[1]
		case 2:
			req.Trechos = trechos[2]
		}

		if req.Trechos == nil {
			continue
		}
		log.Printf("[RESERVA PREPARE] 9.%d", i)

		wg.Add(1)

		go func(url string, req PacoteHttp) {
			log.Printf("[RESERVA PREPARE] 10")

			defer wg.Done()

			body, _ := json.Marshal(req)

			respostaHttp, err := cliente.Post(url+"/reserva/prepare", "application/json", bytes.NewBuffer(body))
			if err != nil || respostaHttp.StatusCode != http.StatusOK {
				log.Printf("[RESERVA PREPARE] 11, erro: %v", err)
				votos <- false //se deu erro, voto é não, cancela tudo
				return
			}
			defer respostaHttp.Body.Close()

			var respostaCorpo PacoteHttp
			json.NewDecoder(respostaHttp.Body).Decode(&respostaCorpo)

			votos <- respostaCorpo.Voto
		}(url, req)
	}
	wg.Wait()

	//apuracao de votos
	commit := true
	for i := 0; i < requisicoes; i++ {
		vote := <-votos
		if !vote {
			commit = false
		}
	}

	log.Printf("[RESERVA PREPARE] 12, commit: %t", commit)

	return transacaoID, commit

}
