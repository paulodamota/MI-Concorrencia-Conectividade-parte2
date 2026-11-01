package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	. "vaijunto/cmd/servidor/protocoloREST"
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
	//constroi a chave dessa reserva
	chave := ReservaChave{ReservaID: reservaID, User: user, Servidor: cfg.ServidorID}

	//prepare
	// o wait group foi dispensavel pq o canal com buffer é bloqueante
	// e sei quantos votos espero como resposta
	votos := make(chan bool, 3)
	numeroDeParticipantes := 0

	for i, url := range cfg.GetUrls() {

		//se o servidor i não participa da reserva, pula
		if !participantes[i] {
			continue
		}
		numeroDeParticipantes++

		go func(url string, ts []TrechoData) {
			//envia pedido de prepare, com os trechos a serem removidos
			status, resp, err := postHTTP(url+"/reserva/remover/prepare", PacoteHttp{
				TransacaoID: transacaoID,
				Trechos:     ts,
			})

			//erro no http cancela transacao
			if err != nil {
				log.Printf("[REMOVER 2PC] erro no prepare para %s: %v", url, err)
				votos <- false
				return
			}

			votos <- (status == http.StatusOK && resp.Voto)

		}(url, trechos[i])
	}

	//resultado da votacao
	commit := true
	for k := 0; k < numeroDeParticipantes; k++ {
		if !<-votos {
			commit = false
		}
	}

	//manda abort se deu errado
	if !commit {
		log.Printf("[REMOVER 2PC] transação %s abortada (algum participante votou não)", transacaoID)
		ReservaAbort(transacaoID, participantes)
		return false, "Não foi possível remover a reserva agora (trecho em uso por outra operação). Tente novamente."
	}

	//++++++++++ DECISÃO ++++++++++
	//remove localmente, se alguém já removeu (ex: motorista cancelou a viagem), aborta
	//isso não deveria acontecer nunca já que a remoção por motorista também usará 2pc
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

	//se cair [AQUI] removeu localmente mas não removeu dos outros servidores
	//preciso fazer um esquema com arquivos para garantir que será guardado e removido quando o servidor retornar

	//++++++++++ FASE 2: COMMIT ++++++++++
	var wg sync.WaitGroup
	for i, url := range cfg.GetUrls() {
		if !participantes[i] {
			continue
		}
		wg.Add(1)
		go func(i int, url string) {
			defer wg.Done()
			//tenta 3 vezes antes de desistir
			for tentativa := 1; tentativa <= 3; tentativa++ {
				status, _, err := postHTTP(url+"/reserva/remover/commit", PacoteHttp{
					TransacaoID: transacaoID,
					Reserva:     chave,
				})

				if err == nil && (status == http.StatusOK || status == http.StatusNotFound) {
					return
				}
				time.Sleep(500 * time.Millisecond)
			}
			log.Printf("[REMOVER RESERVA 2PC] Servidor %d não confirmou commit da transação %s", i+1, transacaoID)
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

	//erro na leitura será tratado como voto false
	var req PacoteHttp
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	//constroi as chaves de acesso aos trechos
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

	//adiciona os trechos ao mapa de transacoes
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

	//reservas a remover: uma só (remoção de reserva) ou várias (cancelamento de viagem)
	alvos := append([]ReservaChave{}, req.Reservas...)
	if req.Reserva.ReservaID != "" {
		alvos = append(alvos, req.Reserva)
	}

	for _, tc := range chaves {
		//acessa a viagem cadastrada pelo motorista que contem aquele trecho
		ViagensMoto.mu.Lock()
		viagem, ok := ViagensMoto.Viagens[tc.UserName][tc.ViagemID]
		ViagensMoto.mu.Unlock()

		//motorista removeu a viagem, não há vaga a devolver para esse trecho
		if !ok {
			continue
		}

		viagem.mu.Lock()
		for _, t := range viagem.Trechos {
			if t.ID != tc.ID {
				continue
			}

			t.mu.Lock()
			//recria o slice de reservas associadas sem a reserva a ser removida
			restantes := make([]ReservaChave, 0, len(t.ReservasAssociadasIDS))

			for _, rc := range t.ReservasAssociadasIDS {
				if contemReserva(alvos, rc) {
					//devolve a vaga
					t.Vagas++
					continue
				}
				restantes = append(restantes, rc)
			}
			//reatribui aqui
			t.ReservasAssociadasIDS = restantes

			t.mu.Unlock()
			break
		}
		viagem.mu.Unlock()
	}

	//remove a transacao dos mapas (libera lock do trecho)
	for _, tc := range chaves {
		delete(transacoes.TransacaoDoTrecho, tc)
	}
	delete(transacoes.TrechosDaTransacao, req.TransacaoID)

	log.Printf("[REMOVER COMMIT] transação %s confirmada", req.TransacaoID)
	w.WriteHeader(http.StatusOK)
}

func contemReserva(lista []ReservaChave, rc ReservaChave) bool {
	for _, a := range lista {
		if a.ReservaID == rc.ReservaID && a.User == rc.User {
			return true
		}
	}
	return false
}
