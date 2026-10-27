package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	. "vaijunto/shared"
)

type Transacoes struct {
	mu sync.Mutex

	//ID da transacao -> chaves de Trechos que fazem parte da transacao
	TrechosDaTransacao map[string][]TrechoChaves

	/*
		Chave do Trecho -> id da transacao
		permite saber se um trecho está em transacao, por exemplo
		caso um motorista cancele a viagem durante a transacao de reserva
	*/
	TransacaoDoTrecho map[TrechoChaves]string
}

//funções da api rest para comunicação entre servidores

type PacoteHttp struct {
	Trechos     []TrechoData `json:"trechos,omitempty"`
	TransacaoID string       `json:"transacaoid,omitempty"`
	Reserva     ReservaChave `json:"passageiro,omitempty"`

	Voto bool `json:"voto,omitempty"`
}

func IniciarServidorREST() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /adjacencias/{cidade}", handleGetAdjacencias)
	mux.HandleFunc("POST /reserva/prepare", handlePostReservaPrepare)
	mux.HandleFunc("POST /reserva/commit", handlePostReservaCommit)

	go func() {
		enderecoPorta := ":" + portas[1]

		log.Printf("##### Servidor [REST] instância '%d' a iniciar na porta %s... #####\n", servidor, enderecoPorta)

		go func() {
			time.Sleep(5 * time.Second)
		}()

		//bloqueante
		if err := http.ListenAndServe(enderecoPorta, mux); err != nil {
			log.Fatalf("Erro no servidor REST: %v", err)
		}
	}()
}

func handleGetAdjacencias(w http.ResponseWriter, r *http.Request) {
	cidade := r.PathValue("cidade")
	adjacencias := grafo.GetAdjacencias(cidade)

	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(PacoteHttp{Trechos: adjacencias})
}

// get apenas por agora
func ComandoParaServidor(comando string, mensagem string, portaDestinatario string) (*PacoteHttp, error) {

	url := GetURLHTTP(servidor) + comando + mensagem

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("[REST] Erro ao fazer requisição HTTP para %s: %v", portaDestinatario, err)
		return nil, err
	}

	var resposta PacoteHttp
	err = json.NewDecoder(resp.Body).Decode(&resposta)
	defer resp.Body.Close()
	if err != nil {
		log.Printf("[REST] Erro ao ler resposta de %s: %v", portaDestinatario, err)
		return nil, err
	}
	return &resposta, nil
}
