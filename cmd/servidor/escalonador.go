package main

import (
	"sort"
	"sync"
	"time"

	. "vaijunto/cmd/servidor/relogioLogico"
)

const (
	janelaOrdenacao = 50 * time.Millisecond
	esperaMaxima    = 130 * time.Millisecond
)

type pedidoReserva struct {
	ts        Timestamp
	id        string // ID da transação
	chaves    []TrechoChaves
	chegada   time.Time
	resultado chan bool // true concedido locks já aplicados, false negado
}

var fila = struct {
	mu      sync.Mutex
	pedidos []*pedidoReserva
}{}

func enfileirar(p *pedidoReserva) {
	fila.mu.Lock()
	defer fila.mu.Unlock()

	fila.pedidos = append(fila.pedidos, p)
	sort.SliceStable(fila.pedidos, func(i, j int) bool {
		return fila.pedidos[i].ts.Antes(fila.pedidos[j].ts)
	})
}

func removerDaFila(p *pedidoReserva) {
	fila.mu.Lock()
	defer fila.mu.Unlock()

	for i, q := range fila.pedidos {
		if q == p {
			fila.pedidos = append(fila.pedidos[:i], fila.pedidos[i+1:]...)
			return
		}
	}
}

// percorre a fila em ordem de timestamp e concede o que for possível
func processarFila() {
	fila.mu.Lock()
	defer fila.mu.Unlock()

	agora := time.Now()
	var esperando []*pedidoReserva //pedidos ainda na fila, todos mais antigos que o atual

	for _, pedido := range fila.pedidos {
		bloqueado := false
		for _, anterior := range esperando {

			//bloqueia caso haja um pedido mais antigo com algum trecho em comum
			if possuiTrechoEmComum(anterior.chaves, pedido.chaves) {
				bloqueado = true
				break
			}
		}

		//se não há pedidos anteriores com o mesmo trecho e ele esperou por mais tempo que a janela
		if !bloqueado && agora.Sub(pedido.chegada) >= janelaOrdenacao {

			//tentar dar prepare o de fato, colocar os trechos nos mapas para esperar o commit
			if concedido, decidido := tentarConceder(pedido); decidido {
				pedido.resultado <- concedido
				continue
			}
		}
		esperando = append(esperando, pedido)
	}

	fila.pedidos = esperando
}

// decidido é false quando o trecho está ocupado, continua esperando
func tentarConceder(p *pedidoReserva) (concedido bool, decidido bool) {
	transacoes.mu.Lock()

	//se o trecho está em outra transação, não concede e mantem esperando na fila
	for _, tc := range p.chaves {
		if _, ocupado := transacoes.TransacaoDoTrecho[tc]; ocupado {
			transacoes.mu.Unlock()
			return false, false
		}
	}
	transacoes.mu.Unlock()

	//viagem removida ou sem vagas, não concede e remove da fila, cancela o pedido
	if !temVaga(p.chaves) {
		return false, true
	}

	//se chega aqui é pq ta livre e tem vaga, aplica trava com timeout
	transacoes.TravarComTimeout(p.id, p.chaves, timeoutTransacaoReserva)
	return true, true
}

func temVaga(chaves []TrechoChaves) bool {

	for _, tc := range chaves {
		ViagensMoto.mu.Lock()
		viagem, ok := ViagensMoto.Viagens[tc.UserName][tc.ViagemID]
		ViagensMoto.mu.Unlock()
		if !ok {
			return false //se não existe
		}

		viagem.mu.Lock()
		achou := false
		for _, t := range viagem.Trechos {
			if t.ID != tc.ID {
				continue
			}
			t.mu.Lock()
			vagas := t.Vagas
			t.mu.Unlock()
			if vagas <= 0 {
				viagem.mu.Unlock()
				return false //achou mas n tem vagas
			}
			achou = true
			break
		}
		viagem.mu.Unlock()

		if !achou {
			return false
		}
	}
	return true //achou e tem vagas
}

func possuiTrechoEmComum(a, b []TrechoChaves) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
		}
	}
	return false
}

func IniciarEscalonador() {
	go func() {
		for {
			time.Sleep(20 * time.Millisecond)
			processarFila()
		}
	}()
}
