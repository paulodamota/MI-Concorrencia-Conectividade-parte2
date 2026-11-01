package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	. "vaijunto/shared"
)

func RemoverViagem2PC(driver string, viagemID string) (bool, string) {
	ViagensMoto.mu.Lock()
	viagem, existe := ViagensMoto.Viagens[driver][viagemID]
	ViagensMoto.mu.Unlock()

	if !existe || viagem == nil {
		return false, "Viagem não encontrada."
	}

	transacaoID := uuid.New().String()
	//ID separado para o prepare dos trechos de outros servidores, senão
	//colidiria com o lock local no mapa de transações (mesmo servidor, mesmo ID)
	txTrechos := transacaoID + ":trechos"

	passageirosFlags := make([]bool, 3)
	trechosFlags := make([]bool, 3)

	abortar := func(motivo string) (bool, string) {
		log.Printf("[REMOVER VIAGEM 2PC] transação %s abortada: %s", transacaoID, motivo)
		abortarPassageiros(transacaoID, passageirosFlags)
		ReservaAbort(txTrechos, trechosFlags)
		liberarTransacaoLocal(transacaoID)
		return false, "Não foi possível remover a viagem agora (" + motivo + "). Tente novamente."
	}

	//++++++++++ FASE 1 ++++++++++

	//1. trava os trechos da viagem. Com eles travados, ninguém consegue
	//   confirmar/remover reservas neles, então a lista de reservas fica estável
	if !travarTrechosLocais(transacaoID, driver, viagem) {
		return false, "Viagem em uso por uma reserva em andamento. Tente novamente."
	}

	//2. reservas afetadas, agrupadas pelo servidor do passageiro
	vistos := make(map[ReservaChave]bool)
	porServidor := make([][]ReservaChave, 3)
	var todas []ReservaChave
	for _, t := range viagem.Trechos {
		t.mu.Lock()
		for _, rc := range t.ReservasAssociadasIDS {
			if rc.Servidor < 1 || rc.Servidor > 3 || vistos[rc] {
				continue
			}
			vistos[rc] = true
			porServidor[rc.Servidor-1] = append(porServidor[rc.Servidor-1], rc)
			todas = append(todas, rc)
		}
		t.mu.Unlock()
	}

	//3. prepare nos servidores de passageiros (vota e devolve os trechos das reservas)
	type voto struct {
		ok      bool
		trechos []TrechoData
	}
	canal := make(chan voto, 3)
	n := 0
	for i, url := range cfg.GetUrls() {
		if len(porServidor[i]) == 0 {
			continue
		}
		passageirosFlags[i] = true
		n++
		go func(url string, rs []ReservaChave) {
			status, resp, err := postHTTP(url+"/viagem/cancelar/prepare", PacoteHttp{
				TransacaoID: transacaoID,
				Reservas:    rs,
			})
			canal <- voto{err == nil && status == http.StatusOK && resp.Voto, resp.Trechos}
		}(url, porServidor[i])
	}

	sim := true
	var trechosDasReservas []TrechoData
	for k := 0; k < n; k++ {
		v := <-canal
		if !v.ok {
			sim = false
		}
		trechosDasReservas = append(trechosDasReservas, v.trechos...)
	}
	if !sim {
		return abortar("servidor de passageiro indisponível")
	}

	//4. prepare nos servidores dos outros trechos dessas reservas
	//   (os trechos desta viagem já estão travados localmente)
	proprios := make(map[string]bool)
	for _, t := range viagem.Trechos {
		proprios[t.ID] = true
	}

	porServidorTrechos := make([][]TrechoData, 3)
	vistosT := make(map[string]bool)
	for _, td := range trechosDasReservas {
		if proprios[td.ID] || vistosT[td.ID] || td.Servidor < 1 || td.Servidor > 3 {
			continue
		}
		vistosT[td.ID] = true
		porServidorTrechos[td.Servidor-1] = append(porServidorTrechos[td.Servidor-1], td)
		trechosFlags[td.Servidor-1] = true
	}

	votos := make(chan bool, 3)
	n = 0
	for i, url := range cfg.GetUrls() {
		if len(porServidorTrechos[i]) == 0 {
			continue
		}
		n++
		go func(url string, ts []TrechoData) {
			status, resp, err := postHTTP(url+"/reserva/remover/prepare", PacoteHttp{
				TransacaoID: txTrechos,
				Trechos:     ts,
			})
			votos <- (err == nil && status == http.StatusOK && resp.Voto)
		}(url, porServidorTrechos[i])
	}

	sim = true
	for k := 0; k < n; k++ {
		if !<-votos {
			sim = false
		}
	}
	if !sim {
		return abortar("trecho de outra viagem em uso")
	}

	//++++++++++ DECISÃO: COMMIT ++++++++++
	//aplica localmente: tira do mapa, do grafo e libera o lock
	ViagensMoto.mu.Lock()
	delete(ViagensMoto.Viagens[driver], viagemID)
	ViagensMoto.mu.Unlock()

	grafo.RemoverViagemMotorista(viagem)
	liberarTransacaoLocal(transacaoID)

	//++++++++++ FASE 2 ++++++++++
	var wg sync.WaitGroup
	for i, url := range cfg.GetUrls() {
		if passageirosFlags[i] {
			wg.Add(1)
			go func(i int, url string) {
				defer wg.Done()
				commitComRetry(url+"/viagem/cancelar/commit", PacoteHttp{
					TransacaoID: transacaoID,
					Motorista:   driver,
				}, transacaoID, i+1)
			}(i, url)
		}
		if trechosFlags[i] {
			wg.Add(1)
			go func(i int, url string) {
				defer wg.Done()
				commitComRetry(url+"/reserva/remover/commit", PacoteHttp{
					TransacaoID: txTrechos,
					Reservas:    todas,
				}, transacaoID, i+1)
			}(i, url)
		}
	}
	wg.Wait()

	log.Printf("[REMOVER VIAGEM 2PC] viagem %s do motorista '%s' removida; %d reservas canceladas (transação %s)",
		viagemID, driver, len(todas), transacaoID)

	return true, fmt.Sprintf("Viagem removida com sucesso. %d reserva(s) de passageiros foram canceladas.", len(todas))
}

// trava (atomicamente) todos os trechos da viagem no mapa de transações deste servidor
func travarTrechosLocais(transacaoID string, driver string, v *Viagem) bool {
	chaves := make([]TrechoChaves, len(v.Trechos))
	for i, t := range v.Trechos {
		chaves[i] = TrechoChaves{UserName: driver, ViagemID: v.ID, ID: t.ID}
	}

	transacoes.mu.Lock()
	defer transacoes.mu.Unlock()

	for _, tc := range chaves {
		if _, ocupado := transacoes.TransacaoDoTrecho[tc]; ocupado {
			return false
		}
	}

	transacoes.TrechosDaTransacao[transacaoID] = chaves
	for _, tc := range chaves {
		transacoes.TransacaoDoTrecho[tc] = transacaoID
	}
	return true
}

func liberarTransacaoLocal(transacaoID string) {
	transacoes.mu.Lock()
	defer transacoes.mu.Unlock()

	for _, tc := range transacoes.TrechosDaTransacao[transacaoID] {
		delete(transacoes.TransacaoDoTrecho, tc)
	}
	delete(transacoes.TrechosDaTransacao, transacaoID)
}

func abortarPassageiros(transacaoID string, servidores []bool) {
	for i, url := range cfg.GetUrls() {
		if servidores[i] {
			go postHTTP(url+"/viagem/cancelar/abort", PacoteHttp{TransacaoID: transacaoID})
		}
	}
}

// envia commit com até 3 tentativas. 404 = participante já aplicou (idempotência)
func commitComRetry(url string, pacote PacoteHttp, transacaoID string, servidorDestino int) {
	for tentativa := 1; tentativa <= 3; tentativa++ {
		status, _, err := postHTTP(url, pacote)
		if err == nil && (status == http.StatusOK || status == http.StatusNotFound) {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	log.Printf("[REMOVER VIAGEM 2PC] ATENÇÃO: servidor %d não confirmou commit (%s) da transação %s",
		servidorDestino, url, transacaoID)
}

// reservas "preparadas" para cancelamento: ID da transação -> reservas
var cancelamentos = struct {
	mu           sync.Mutex
	porTransacao map[string][]ReservaChave
}{porTransacao: make(map[string][]ReservaChave)}

// prepare: registra as reservas e devolve os trechos delas ao coordenador
func handlePostViagemCancelarPrepare(w http.ResponseWriter, r *http.Request) {
	var req PacoteHttp
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var trechos []TrechoData
	ReservasPass.mu.Lock()
	for _, rc := range req.Reservas {
		if reserva, ok := ReservasPass.Reservas[rc.User][rc.ReservaID]; ok && reserva != nil {
			trechos = append(trechos, reserva.Trechos...)
		}
	}
	ReservasPass.mu.Unlock()

	cancelamentos.mu.Lock()
	cancelamentos.porTransacao[req.TransacaoID] = req.Reservas
	cancelamentos.mu.Unlock()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(PacoteHttp{Voto: true, Trechos: trechos})
}

// commit: apaga as reservas e notifica os passageiros
func handlePostViagemCancelarCommit(w http.ResponseWriter, r *http.Request) {
	var req PacoteHttp
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cancelamentos.mu.Lock()
	reservas, existe := cancelamentos.porTransacao[req.TransacaoID]
	delete(cancelamentos.porTransacao, req.TransacaoID)
	cancelamentos.mu.Unlock()

	if !existe {
		http.Error(w, "Transação não encontrada", http.StatusNotFound)
		return
	}

	ReservasPass.mu.Lock()
	for _, rc := range reservas {
		delete(ReservasPass.Reservas[rc.User], rc.ReservaID) //delete em mapa nil é seguro
	}
	ReservasPass.mu.Unlock()

	for _, rc := range reservas {
		if p, ok := Passageiros.Load(rc.User); ok {
			msg := fmt.Sprintf("Sua reserva de ID '%s' foi cancelada pois o motorista '%s' cancelou a viagem!\n Em breve você receberá reembolso de sua reserva. Agradecemos a compreensão!\n", rc.ReservaID, req.Motorista)
			p.(*Usuario).AdicionarNotificacao(msg)
		}
	}

	log.Printf("[CANCELAR VIAGEM COMMIT] transação %s: %d reservas removidas", req.TransacaoID, len(reservas))
	w.WriteHeader(http.StatusOK)
}

func handlePostViagemCancelarAbort(w http.ResponseWriter, r *http.Request) {
	var req PacoteHttp
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cancelamentos.mu.Lock()
	delete(cancelamentos.porTransacao, req.TransacaoID)
	cancelamentos.mu.Unlock()

	w.WriteHeader(http.StatusOK)
}
