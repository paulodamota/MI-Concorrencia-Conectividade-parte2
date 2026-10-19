package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

//funções da api rest para comunicação entre servidores

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

// Handler HTTP REST para atender requisições dos outros servidores
func handleBusca(w http.ResponseWriter, r *http.Request) {
	origem := r.PathValue("origem")
	destino := r.PathValue("destino")
	data := r.PathValue("data")

	log.Printf("[REST] Requisição de busca recebida para o trecho: %s -> %s em %s", origem, destino, data)

	// Consulta o grafo local da empresa
	itinerarios := grafo.BuscarRotas(origem, destino, data)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "SUCESSO",
		"itinerarios": itinerarios,
	})
}

func handlePreReservaExterna(w http.ResponseWriter, r *http.Request) {
	// Lógica para receber a tentativa de pré-reserva de outro servidor (exigência do Problema 2)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "SUCESSO", "mensagem": "Pré-reserva registrada"})
}

func handlePing(w http.ResponseWriter, r *http.Request) {
	// Captura o parâmetro dinâmico da URL
	texto := r.PathValue("texto")

	// IMPRIME NO TERMINAL DO SERVIDOR QUE RECEBEU O PING
	log.Printf("[REST] Ping recebido com o texto: %s", texto)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "SUCESSO",
		"mensagem": fmt.Sprintf("Pong! %s", texto),
	})
}
