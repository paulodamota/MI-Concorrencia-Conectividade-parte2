package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	. "vaijunto/shared"
)

func RemoverReserva2PC(user string, reservaID string) (bool, string) {
	//localiza a reserva
	ReservasPass.mu.Lock()
	reserva, existe := ReservasPass.Reservas[user][reservaID]
	ReservasPass.mu.Unlock()

	if !existe || reserva == nil {
		return false, "Reserva não encontrada."
	}

	//agrupa os trechos por servidor (usa o Servidor de cada trecho,
	//sem depender de reserva.Servidores)
	trechos := make([][]TrechoData, 3)
	participantes := make([]bool, 3)
	for _, t := range reserva.Trechos {
		if t.Servidor < 1 || t.Servidor > 3 {
			continue
		}
		trechos[t.Servidor-1] = append(trechos[t.Servidor-1], t)
		participantes[t.Servidor-1] = true
	}

	transacaoID := uuid.New().String()
	chave := ReservaChave{ReservaID: reservaID, User: user, Servidor: servidor}

	//++++++++++ FASE 1: PREPARE ++++++++++
	votos := make(chan bool, 3)
	n := 0
	for i, url := range urlsDosServidores {
		if !participantes[i] {
			continue
		}
		n++
		go func(url string, ts []TrechoData) {
			status, resp, err := postHTTP(url+"/reserva/remover/prepare", PacoteHttp{
				TransacaoID: transacaoID,
				Trechos:     ts,
				Reserva:     chave,
			})
			if err != nil {
				log.Printf("[REMOVER 2PC] erro no prepare para %s: %v", url, err)
				votos <- false
				return
			}
			votos <- (status == http.StatusOK && resp.Voto)
		}(url, trechos[i])
	}

	commit := true
	for k := 0; k < n; k++ {
		if !<-votos {
			commit = false
		}
	}

	if !commit {
		log.Printf("[REMOVER 2PC] transação %s abortada (algum participante votou não)", transacaoID)
		ReservaAbort(transacaoID, participantes)
		return false, "Não foi possível remover a reserva agora (trecho em uso por outra operação). Tente novamente."
	}

	//++++++++++ DECISÃO ++++++++++
	//remove localmente, se alguém já removeu (ex: motorista cancelou a viagem), aborta
	ReservasPass.mu.Lock()
	_, aindaExiste := ReservasPass.Reservas[user][reservaID]
	if aindaExiste {
		delete(ReservasPass.Reservas[user], reservaID)
	}
	ReservasPass.mu.Unlock()

	if !aindaExiste {
		ReservaAbort(transacaoID, participantes)
		return false, "Reserva não encontrada."
	}

	//++++++++++ FASE 2: COMMIT ++++++++++
	var wg sync.WaitGroup
	for i, url := range urlsDosServidores {
		if !participantes[i] {
			continue
		}
		wg.Add(1)
		go func(i int, url string) {
			defer wg.Done()
			for tentativa := 1; tentativa <= 3; tentativa++ {
				status, _, err := postHTTP(url+"/reserva/remover/commit", PacoteHttp{
					TransacaoID: transacaoID,
					Reserva:     chave,
				})

				if err == nil && (status == http.StatusOK || status == http.StatusNotFound) {
					return
				}
				time.Sleep(time.Duration(tentativa) * 500 * time.Millisecond)
			}
			log.Printf("[REMOVER 2PC] ATENÇÃO: servidor %d não confirmou commit da transação %s", i+1, transacaoID)
		}(i, url)
	}
	wg.Wait()

	log.Printf("Reserva %s do passageiro '%s' removida (transação %s)", reservaID, user, transacaoID)
	return true, "Reserva removida com sucesso"
}

// funcao de post pra facilitar
func postHTTP(url string, pacote PacoteHttp) (int, PacoteHttp, error) {
	var resposta PacoteHttp
	body, _ := json.Marshal(pacote)

	resp, err := cliente.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return 0, resposta, err
	}
	defer resp.Body.Close()

	json.NewDecoder(resp.Body).Decode(&resposta)
	return resp.StatusCode, resposta, nil
}

// trava os trechos da transação e vota
func handlePostRemoverPrepare(w http.ResponseWriter, r *http.Request) {
	var req PacoteHttp
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	chaves := make([]TrechoChaves, len(req.Trechos))
	for i, td := range req.Trechos {
		chaves[i] = TrechoChaves{UserName: td.Motorista, ViagemID: td.ViagemID, ID: td.ID}
	}

	transacoes.mu.Lock()
	defer transacoes.mu.Unlock()

	//se algum trecho já está em outra transação vota false
	for _, tc := range chaves {
		if outra, ocupado := transacoes.TransacaoDoTrecho[tc]; ocupado {
			log.Printf("[REMOVER PREPARE] trecho %s ocupado pela transação %s", tc.ID, outra)
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(PacoteHttp{Voto: false})
			return
		}
	}

	//trava os trechos
	transacoes.TrechosDaTransacao[req.TransacaoID] = chaves
	for _, tc := range chaves {
		transacoes.TransacaoDoTrecho[tc] = req.TransacaoID
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(PacoteHttp{Voto: true})
}

// devolve a vaga, tira a reserva do trecho e libera o lock
func handlePostRemoverCommit(w http.ResponseWriter, r *http.Request) {
	var req PacoteHttp
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	transacoes.mu.Lock()
	defer transacoes.mu.Unlock()

	chaves, existe := transacoes.TrechosDaTransacao[req.TransacaoID]
	if !existe {
		http.Error(w, "Transação não encontrada", http.StatusNotFound)
		return
	}

	for _, tc := range chaves {
		ViagensMoto.mu.Lock()
		viagem, ok := ViagensMoto.Viagens[tc.UserName][tc.ViagemID]
		ViagensMoto.mu.Unlock()

		//motorista removeu a viagem: não há vaga a devolver
		if !ok {
			continue
		}

		viagem.mu.Lock()
		for _, t := range viagem.Trechos {
			if t.ID != tc.ID {
				continue
			}

			t.mu.Lock()
			//cria slice novo (não altera o array que pode estar sendo serializado)
			restantes := make([]ReservaChave, 0, len(t.ReservasAssociadasIDS))
			removido := false
			for _, rc := range t.ReservasAssociadasIDS {
				if rc.ReservaID == req.Reserva.ReservaID && rc.User == req.Reserva.User {
					removido = true
					continue
				}
				restantes = append(restantes, rc)
			}
			t.ReservasAssociadasIDS = restantes
			//só devolve a vaga se a reserva realmente ocupava esse trecho
			if removido {
				t.Vagas++
			}
			t.mu.Unlock()
			break
		}
		viagem.mu.Unlock()
	}

	for _, tc := range chaves {
		delete(transacoes.TransacaoDoTrecho, tc)
	}
	delete(transacoes.TrechosDaTransacao, req.TransacaoID)

	log.Printf("[REMOVER COMMIT] transação %s confirmada", req.TransacaoID)
	w.WriteHeader(http.StatusOK)
}
