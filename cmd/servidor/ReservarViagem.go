package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/google/uuid"

	. "vaijunto/shared"
)

// tenta reservar vagas em trechos
func (g *GrafoRotas) ReservarViagem(viagemData *ViagemData) (string, bool) {
	if len(viagemData.Trechos) == 0 {
		return "", false
	}

	transacaoID := uuid.New().String()

	//fazer essa separação já ao montar ViagemData?
	var trechosDoServidor_1 []TrechoData
	var trechosDoServidor_2 []TrechoData
	var trechosDoServidor_3 []TrechoData

	for _, trechoData := range viagemData.Trechos {
		switch trechoData.Servidor {
		case 1:
			trechosDoServidor_1 = append(trechosDoServidor_1, trechoData)
		case 2:
			trechosDoServidor_2 = append(trechosDoServidor_2, trechoData)
		case 3:
			trechosDoServidor_3 = append(trechosDoServidor_3, trechoData)
		}
	}

	var wg sync.WaitGroup

	//++[ ENVIO DE PREPARE ]++
	votos := make(chan bool, len(participantes))

	for i, url := range participantes {

		var req PacoteHttp
		req.TransacaoID = transacaoID

		//construindo requisicao com os trechos desejados
		switch i + 1 {
		case 1:
			req.Trechos = trechosDoServidor_1
		case 2:
			req.Trechos = trechosDoServidor_2
		case 3:
			req.Trechos = trechosDoServidor_3
		}

		wg.Add(1)

		go func(url string, req PacoteHttp) {
			defer wg.Done()

			body, _ := json.Marshal(req)

			respostaHttp, err := http.Post(url+"/reserva/prepare", "application/json", bytes.NewBuffer(body))
			if err != nil || respostaHttp.StatusCode != http.StatusOK {
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
	for i := 0; i < len(participantes); i++ {
		vote := <-votos
		if !vote {
			commit = false
		}
	}

	return transacaoID, commit

}
