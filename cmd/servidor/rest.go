package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	. "vaijunto/cmd/servidor/protocoloREST"
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

func InicializarTransacoes() *Transacoes {
	return &Transacoes{
		TrechosDaTransacao: make(map[string][]TrechoChaves),
		TransacaoDoTrecho:  make(map[TrechoChaves]string),
	}
}

//funções da api rest para comunicação entre servidores

var (
	// cliente HTTP para fazer requisições com timeout
	cliente *http.Client
)

func IniciarServidorREST() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /adjacencias/{cidade}", handleGetAdjacencias)
	mux.HandleFunc("POST /reserva/prepare", handlePostReservaPrepare)
	mux.HandleFunc("POST /reserva/commit", handlePostReservaCommit)
	mux.HandleFunc("POST /reserva/abort", handlePostReservaAbort)

	mux.HandleFunc("POST /reserva/remover/prepare", handlePostRemoverPrepare)
	mux.HandleFunc("POST /reserva/remover/commit", handlePostRemoverCommit)

	mux.HandleFunc("POST /viagem/cancelar/prepare", handlePostViagemCancelarPrepare)
	mux.HandleFunc("POST /viagem/cancelar/commit", handlePostViagemCancelarCommit)
	mux.HandleFunc("POST /viagem/cancelar/abort", handlePostViagemCancelarAbort)

	//ao enviar requisição, espera por 200 milissegundos
	//não havendo resposta retorna erro
	cliente = &http.Client{
		Timeout:   200 * time.Millisecond,
		Transport: relogioDeLamport.Transporte(http.DefaultTransport),
	}

	IniciarEscalonador()

	go func() {
		enderecoPorta := ":" + cfg.PortaHTTP

		log.Printf("##### Servidor [REST] instância '%d' a iniciar na porta %s... #####\n", cfg.ServidorID, enderecoPorta)

		go func() {
			time.Sleep(3 * time.Second)
		}()

		//bloqueante
		if err := http.ListenAndServe(enderecoPorta, relogioDeLamport.Middleware(mux)); err != nil {
			log.Fatalf("Erro no servidor REST: %v", err)
		}
	}()
}

func handleGetAdjacencias(w http.ResponseWriter, r *http.Request) {

	cidade := r.PathValue("cidade")

	log.Printf("Requisição recebida para a cidade: %s", cidade)

	adjacencias := grafo.GetAdjacencias(cidade)

	log.Printf("Adjacencias para %s: %v", cidade, adjacencias)

	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(PacoteHttp{Trechos: adjacencias})
}

// get apenas por agora
func ComandoParaServidor(comando string, mensagem string, Destinatario int) (*PacoteHttp, error) {

	url := cfg.URLHTTP(Destinatario) + comando + mensagem

	log.Printf("\n url: %s", url)

	resp, err := cliente.Get(url)
	if err != nil {
		log.Printf("[REST] Erro ao fazer requisição HTTP para %d: %v", Destinatario, err)
		return nil, err
	}

	var resposta PacoteHttp
	err = json.NewDecoder(resp.Body).Decode(&resposta)
	defer resp.Body.Close()
	if err != nil {
		log.Printf("[REST] Erro ao ler resposta de %d: %v", Destinatario, err)
		return nil, err
	}
	return &resposta, nil
}
