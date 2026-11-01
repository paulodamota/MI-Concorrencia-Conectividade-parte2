// relogio logico
package relogio

import "sync"

//criado uma vez no servidor que recebeu a requisição do cliente
//vai junto com a transação
type Timestamp struct {
	Tempo    int64 `json:"tempo"`
	Servidor int   `json:"servidor"`
}

// retorna true se a vem antes de b na ordem total
func (a Timestamp) Antes(b Timestamp) bool {
	if a.Tempo != b.Tempo {
		return a.Tempo < b.Tempo
	}
	return a.Servidor < b.Servidor
}

type Lamport struct {
	mu         sync.Mutex
	tempo      int64
	servidorID int
}

func New(servidorID int) *Lamport {
	return &Lamport{servidorID: servidorID}
}

//evento local / envio
func (l *Lamport) Evento() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.tempo++
	return l.tempo
}

//recebimento de mensagem com o carimbo t
func (l *Lamport) Receber(t int64) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	if t > l.tempo {
		l.tempo = t
	}
	l.tempo++
	return l.tempo
}

func (l *Lamport) Carimbo() Timestamp {
	return Timestamp{Tempo: l.Evento(), Servidor: l.servidorID}
}
