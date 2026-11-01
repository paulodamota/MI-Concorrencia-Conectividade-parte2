package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

func handlePostReservaPrepare(w http.ResponseWriter, r *http.Request) {
	var req PacoteHttp
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	chaves := make([]TrechoChaves, len(req.Trechos))
	for i, td := range req.Trechos {
		chaves[i] = TrechoChaves{UserName: td.Motorista, ViagemID: td.ViagemID, ID: td.ID}
	}

	pedido := &pedidoReserva{
		ts:        req.Timestamp,
		id:        req.TransacaoID,
		chaves:    chaves,
		chegada:   time.Now(),
		resultado: make(chan bool, 1),
	}
	log.Printf("[HANDLE RESERVA PREPARE 1]")

	enfileirar(pedido)
	log.Printf("[HANDLE RESERVA PREPARE 2]")

	select {
	case ok := <-pedido.resultado:
		log.Printf("[HANDLE RESERVA PREPARE 3]")
		votarPrepare(w, req, ok)

	case <-time.After(esperaMaxima):
		log.Printf("[HANDLE RESERVA PREPARE 4]")

		removerDaFila(pedido)
		//pode ter sido concedido no instante do timeout se sim, os locks já estão aplicados
		select {
		case ok := <-pedido.resultado:
			votarPrepare(w, req, ok)
		default:
			log.Printf("[PREPARE] transação %s (ts %+v) expirou na fila", req.TransacaoID, req.Timestamp)
			votarPrepare(w, req, false)
		}
	}
}

func votarPrepare(w http.ResponseWriter, req PacoteHttp, ok bool) {
	log.Printf("[HANDLE RESERVA PREPARE 5]")

	if ok {
		log.Printf("[PREPARE] transação %s (ts %+v) concedida", req.TransacaoID, req.Timestamp)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(PacoteHttp{Voto: true})
		return
	}
	w.WriteHeader(http.StatusConflict)
	json.NewEncoder(w).Encode(PacoteHttp{Voto: false})
}
