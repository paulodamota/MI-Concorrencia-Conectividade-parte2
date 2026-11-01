package main

import (
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"
)

type RelogioLamport struct {
	mu    sync.Mutex
	tempo int64
}

var relogio = &RelogioLamport{}

// evento local / envio
func (r *RelogioLamport) Evento() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tempo++
	return r.tempo
}

// recebimento de mensagem com o carimbo t
func (r *RelogioLamport) Receber(t int64) int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t > r.tempo {
		r.tempo = t
	}
	r.tempo++
	return r.tempo
}

/*
carimbo de uma requisição de usuário. É criado UMA vez, no servidor que recebeu
a requisição do cliente, e viaja junto com a transação até os participantes.
*/
type Timestamp struct {
	Tempo    int64 `json:"tempo"`
	Servidor int   `json:"servidor"`
}

func (r *RelogioLamport) Carimbo() Timestamp {
	return Timestamp{Tempo: r.Evento(), Servidor: servidor}
}

// true se a vem antes de b na ordem total
func (a Timestamp) Antes(b Timestamp) bool {
	if a.Tempo != b.Tempo {
		return a.Tempo < b.Tempo
	}
	return a.Servidor < b.Servidor
}

// cliente: carimba toda requisição enviada e atualiza o relógio com a resposta
type transporteRelogio struct{ base http.RoundTripper }

func (t transporteRelogio) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context()) //RoundTripper não deve alterar a requisição original
	req.Header.Set("X-Relogio", strconv.FormatInt(relogio.Evento(), 10))

	resp, err := t.base.RoundTrip(req)
	if err == nil {
		if v, e := strconv.ParseInt(resp.Header.Get("X-Relogio"), 10, 64); e == nil {
			relogio.Receber(v)
		}
	}
	return resp, err
}

// servidor: atualiza o relógio ao receber e devolve o valor no header da resposta
func comRelogio(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var t int64
		if v, err := strconv.ParseInt(r.Header.Get("X-Relogio"), 10, 64); err == nil {
			t = v
		}
		w.Header().Set("X-Relogio", strconv.FormatInt(relogio.Receber(t), 10))
		next.ServeHTTP(w, r)
	})
}

const (
	janelaOrdenacao = 50 * time.Millisecond  // assume que o atraso de rede entre servidores é menor que isso
	esperaMaxima    = 130 * time.Millisecond // menor que o timeout de 2s do cliente HTTP
)

type pedidoReserva struct {
	ts        Timestamp
	id        string // ID da transação
	chaves    []TrechoChaves
	chegada   time.Time
	resultado chan bool // true = concedido (locks já aplicados), false = negado
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
// escalonador
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

// decidido=false: trecho ocupado, continua esperando
func tentarConceder(p *pedidoReserva) (concedido bool, decidido bool) {
	transacoes.mu.Lock()
	defer transacoes.mu.Unlock()

	for _, tc := range p.chaves {
		if _, ocupado := transacoes.TransacaoDoTrecho[tc]; ocupado {
			return false, false
		}
	}

	//viagem removida ou sem vagas: NÃO definitivo
	if !temVaga(p.chaves) {
		return false, true
	}

	transacoes.TrechosDaTransacao[p.id] = p.chaves
	for _, tc := range p.chaves {
		transacoes.TransacaoDoTrecho[tc] = p.id
	}
	return true, true
}

func temVaga(chaves []TrechoChaves) bool {
	for _, tc := range chaves {
		ViagensMoto.mu.Lock()
		viagem, ok := ViagensMoto.Viagens[tc.UserName][tc.ViagemID]
		ViagensMoto.mu.Unlock()
		if !ok {
			return false
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
				return false
			}
			achou = true
			break
		}
		viagem.mu.Unlock()

		if !achou {
			return false
		}
	}
	return true
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

// chamar uma vez no main (ou dentro de IniciarServidorREST)
func IniciarEscalonador() {
	go func() {
		for {
			time.Sleep(20 * time.Millisecond)
			processarFila()
		}
	}()
}
