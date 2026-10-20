package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	. "vaijunto/shared"
)

//funções da api rest para comunicação entre servidores

type RespostaHttp struct {
	Adjacencias []TrechoData `json:"adjacencias,omitempty"`
}

func handler(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if ok {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "origem: %s\ndestino: %s", id[0], id[1])
	}
}

func parseID(r *http.Request) ([]string, bool) {
	o := r.PathValue("origem")
	d := r.PathValue("destino")
	if o == "" || d == "" {
		return nil, false
	}
	return []string{o, d}, true
}

func handleGetAdjacencias(w http.ResponseWriter, r *http.Request) {
	cidade := r.PathValue("cidade")
	adjacencias := grafo.GetAdjacencias(cidade)

	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(RespostaHttp{Adjacencias: adjacencias})
}

func handleBusca(w http.ResponseWriter, r *http.Request) {
	origem := r.PathValue("origem")
	destino := r.PathValue("destino")
	data := r.PathValue("data")

	log.Printf("[REST] Requisição de busca recebida para o trecho: %s -> %s em %s", origem, destino, data)

	itinerarios := grafo.BuscarRotas(origem, destino, data)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"itinerarios": itinerarios,
	})
}

func handlePreReservaExterna(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json") //inutil pra mim, mas deixa ai
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "SUCESSO", "mensagem": "Pré-reserva registrada"})
}

func handlePing(w http.ResponseWriter, r *http.Request) {
	texto := r.PathValue("texto")

	log.Printf("[REST] Ping recebido com o texto: %s", texto)

	w.Header().Set("Content-Type", "application/json")
	//definir um padrão de resposta para as requisições
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "SUCESSO",
		"mensagem": fmt.Sprintf("Pong! %s", texto),
	})
}

func IniciarServidorREST() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /buscar/{origem}/{destino}/{data}", handleBusca)
	mux.HandleFunc("GET /adjacencias/{cidade}", handleGetAdjacencias)
	mux.HandleFunc("POST /reservar/{origem}/{destino}/{data}", handleBusca)
	mux.HandleFunc("GET /ping/{texto}", handlePing)

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
