package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	. "vaijunto/shared"
)

//funções da api rest para comunicação entre servidores

type RespostaHttp struct {
	Adjacencias []TrechoData `json:"adjacencias,omitempty"`
}

func IniciarServidorREST() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /adjacencias/{cidade}", handleGetAdjacencias)

	go func() {
		enderecoPorta := ":" + portas[1]

		log.Printf("##### Servidor [REST] instância '%s' a iniciar na porta %s... #####\n", servidor, enderecoPorta)

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

	json.NewEncoder(w).Encode(RespostaHttp{Adjacencias: adjacencias})
}

func GetAdjacenciasTodosServidores(comando string, mensagem string) {
	switch servidor {
	case 1:
		ComandoParaServidor(comando, mensagem, GetPorta(2)[1])
		ComandoParaServidor(comando, mensagem, GetPorta(3)[1])
	case 2:
		ComandoParaServidor(comando, mensagem, GetPorta(1)[1])
		ComandoParaServidor(comando, mensagem, GetPorta(3)[1])
	case 3:
		ComandoParaServidor(comando, mensagem, GetPorta(1)[1])
		ComandoParaServidor(comando, mensagem, GetPorta(2)[1])
	default:
		log.Printf("Servidor não reconhecido: %d", servidor)
	}
}

// get apenas por agora
func ComandoParaServidor(comando string, mensagem string, portaDestinatario string) (*RespostaHttp, error) {

	url := "http://" + endereco + ":" + portaDestinatario + comando + mensagem

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("[REST] Erro ao fazer requisição HTTP para %s: %v", portaDestinatario, err)
		return nil, err
	}

	var resposta RespostaHttp
	err = json.NewDecoder(resp.Body).Decode(&resposta)
	defer resp.Body.Close()
	if err != nil {
		log.Printf("[REST] Erro ao ler resposta de %s: %v", portaDestinatario, err)
		return nil, err
	}
	return &resposta, nil
}

//não aguento mais esse pbl socorro meu deus que problema chato
