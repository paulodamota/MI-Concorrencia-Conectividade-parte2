package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	. "vaijunto/shared"
)

// estados da transacao no log do coordenador
type TipoWAL string

const (
	WALPrepared  TipoWAL = "PREPARED"  // prepare ok, esperando o passageiro responder
	WALCommitted TipoWAL = "COMMITTED" // passageiro confirmou, vai commitar
	WALDone      TipoWAL = "DONE"      // terminou tudo ou cancelou, pode ignorar
	WALAck       TipoWAL = "ACK"       // servidor especifico confirmou o commit
)

// caminho do arquivo wal do coordenador por servidor
func caminhoWAL() string {
	if cfg != nil && cfg.ServidorID > 0 {
		return fmt.Sprintf("dados/wal_reservas_srv%d.jsonl", cfg.ServidorID)
	}
	return "dados/wal_reservas.jsonl"
}

// dados de cada linha salva no log
type EntradaWAL struct {
	Tipo       TipoWAL     `json:"tipo"`
	TxID       string      `json:"tx_id"`
	Passageiro string      `json:"passageiro,omitempty"`
	Servidores []bool      `json:"servidores,omitempty"`
	Viagem     *ViagemData `json:"viagem,omitempty"`
	Servidor   int         `json:"servidor,omitempty"` // id do servidor que deu ack (1, 2 ou 3)
	Timestamp  time.Time   `json:"timestamp"`
}

var walMu sync.Mutex
var walArquivo *os.File

// abre ou cria o arquivo de log do coordenador
func InicializarWAL() {
	// cria pasta dados se nao existir
	if err := os.MkdirAll("dados", 0755); err != nil {
		log.Fatalf("[WAL] Falha ao criar diretório de dados: %v", err)
	}

	caminho := caminhoWAL()
	var err error
	walArquivo, err = os.OpenFile(caminho, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("[WAL] Falha ao abrir arquivo de log: %v", err)
	}
	log.Println("[WAL] Log de transações inicializado:", caminho)
}

// escreve no log e forca salvar no disco
func GravarWAL(entrada EntradaWAL) {
	entrada.Timestamp = time.Now()
	linha, err := json.Marshal(entrada)
	if err != nil {
		log.Printf("[WAL] Erro ao serializar entrada: %v", err)
		return
	}

	walMu.Lock()
	defer walMu.Unlock()

	if walArquivo == nil {
		return
	}

	if _, err = walArquivo.Write(append(linha, '\n')); err != nil {
		log.Printf("[WAL] Erro ao gravar no log: %v", err)
		return
	}
	// sync pra garantir que foi pro disco
	if err = walArquivo.Sync(); err != nil {
		log.Printf("[WAL] Erro no fsync do log: %v", err)
	}
}

// le o log pra recuperar transacoes pendentes apos reiniciar
func RecuperarWAL() {
	caminho := caminhoWAL()
	f, err := os.Open(caminho)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Printf("[WAL] Erro ao abrir log para recovery: %v", err)
		return
	}
	defer f.Close()

	// reconstroi o estado lendo linha por linha
	estados := make(map[string]EntradaWAL)
	acks := make(map[string]map[int]bool) // quais servidores ja confirmaram commit dessa tx

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e EntradaWAL
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			log.Printf("[WAL] Linha inválida ignorada durante recovery: %v", err)
			continue
		}
		switch e.Tipo {
		case WALPrepared:
			estados[e.TxID] = e
		case WALCommitted:
			estados[e.TxID] = e
		case WALAck:
			if acks[e.TxID] == nil {
				acks[e.TxID] = make(map[int]bool)
			}
			acks[e.TxID][e.Servidor] = true
		case WALDone:
			delete(estados, e.TxID)
			delete(acks, e.TxID)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("[WAL] Erro ao ler log durante recovery: %v", err)
	}

	if len(estados) == 0 {
		log.Println("[WAL] Nenhuma transação pendente encontrada no log.")
		return
	}

	log.Printf("[WAL] %d transação(ões) pendente(s) encontrada(s) no log — iniciando recovery.", len(estados))

	for _, e := range estados {
		switch e.Tipo {
		case WALPrepared:
			// caiu antes do passageiro confirmar, entao aborta
			log.Printf("[WAL RECOVERY] Abortando tx %s (estava em PREPARED — nenhuma decisão tomada)", e.TxID)
			ReservaAbort(e.TxID, e.Servidores)
			GravarWAL(EntradaWAL{Tipo: WALDone, TxID: e.TxID})

		case WALCommitted:
			// ja tinha decidido commitar, reenvia commit pros servidores que faltam
			log.Printf("[WAL RECOVERY] Reenviando commits da tx %s (estava em COMMITTED)", e.TxID)
			entrada := e

			// filtra so os servidores que ainda nao confirmaram
			pendentes := make([]bool, len(entrada.Servidores))
			for i, participa := range entrada.Servidores {
				pendentes[i] = participa && !acks[entrada.TxID][i+1]
			}

			go recuperarCommit(entrada, pendentes)
		}
	}
}

// recupera transacao que ja tinha commit decidido
func recuperarCommit(e EntradaWAL, pendentes []bool) {
	// restaura a reserva local se ainda nao tiver
	if e.Viagem != nil {
		ReservasPass.mu.Lock()
		if ReservasPass.Reservas[e.Passageiro] == nil {
			ReservasPass.Reservas[e.Passageiro] = InicializarReservasByUser()
		}
		if _, jaExiste := ReservasPass.Reservas[e.Passageiro][e.TxID]; !jaExiste {
			copia := *e.Viagem
			copia.ID = e.TxID
			ReservasPass.Reservas[e.Passageiro][e.TxID] = &copia
			log.Printf("[WAL RECOVERY] Reserva %s restaurada localmente para passageiro '%s'", e.TxID, e.Passageiro)
		}
		ReservasPass.mu.Unlock()
	}

	// fica tentando ate todos confirmarem (espera 3s pro rest subir)
	retryPersistente(e.TxID, e.Passageiro, pendentes, 3*time.Second)
	log.Printf("[WAL RECOVERY] Tx %s concluída com sucesso no recovery.", e.TxID)
}

// log do participante pra nao perder trechos preparados se reiniciar
func caminhoWALParticipante() string {
	if cfg != nil && cfg.ServidorID > 0 {
		return fmt.Sprintf("dados/wal_participante_srv%d.jsonl", cfg.ServidorID)
	}
	return "dados/wal_participante.jsonl"
}

type EntradaWALParticipante struct {
	Tipo      string         `json:"tipo"`
	TxID      string         `json:"tx_id"`
	Chaves    []TrechoChaves `json:"chaves,omitempty"` // trechos que foram travados
	Timestamp time.Time      `json:"timestamp"`
}

var (
	walPartMu      sync.Mutex
	walPartArquivo *os.File
)

// guarda os trechos preparados que ainda nao receberam commit
var pendentesCommit = struct {
	mu  sync.Mutex
	txs map[string][]TrechoChaves
}{txs: make(map[string][]TrechoChaves)}

// abre o arquivo de log do participante
func InicializarWALParticipante() {
	caminho := caminhoWALParticipante()
	var err error
	walPartArquivo, err = os.OpenFile(caminho, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("[WAL PART] Falha ao abrir arquivo de log do participante: %v", err)
	}
	log.Println("[WAL PART] Log do participante inicializado:", caminho)
}

// salva acao do participante no log
func GravarWALParticipante(entrada EntradaWALParticipante) {
	entrada.Timestamp = time.Now()
	linha, err := json.Marshal(entrada)
	if err != nil {
		log.Printf("[WAL PART] Erro ao serializar entrada: %v", err)
		return
	}

	walPartMu.Lock()
	defer walPartMu.Unlock()

	if walPartArquivo == nil {
		return
	}

	if _, err = walPartArquivo.Write(append(linha, '\n')); err != nil {
		log.Printf("[WAL PART] Erro ao gravar no log do participante: %v", err)
		return
	}
	if err = walPartArquivo.Sync(); err != nil {
		log.Printf("[WAL PART] Erro no fsync do log do participante: %v", err)
	}
}

// le o log do participante pra recuperar trechos que tavam preparados
func RecuperarWALParticipante() {
	caminho := caminhoWALParticipante()
	f, err := os.Open(caminho)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Printf("[WAL PART] Erro ao abrir log do participante para recovery: %v", err)
		return
	}
	defer f.Close()

	estados := make(map[string]EntradaWALParticipante)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e EntradaWALParticipante
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			log.Printf("[WAL PART] Linha inválida ignorada: %v", err)
			continue
		}
		switch e.Tipo {
		case "PREPARED":
			estados[e.TxID] = e
		case "COMMITTED":
			delete(estados, e.TxID)
			// ja foi comitado antes, marca pra nao duplicar vaga se receber commit de novo
			commitadas.mu.Lock()
			commitadas.ids[e.TxID] = true
			commitadas.mu.Unlock()
		case "ABORTED":
			delete(estados, e.TxID)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("[WAL PART] Erro de leitura no recovery do participante: %v", err)
	}

	if len(estados) == 0 {
		log.Println("[WAL PART] Nenhuma transação pendente de participante encontrada.")
		return
	}

	log.Printf("[WAL PART] %d transação(ões) em PREPARED encontrada(s) — aguardando commit do coordenador.", len(estados))

	// coloca no mapa de pendentes pro handlePostReservaCommit achar
	pendentesCommit.mu.Lock()
	for txID, e := range estados {
		pendentesCommit.txs[txID] = e.Chaves
		log.Printf("[WAL PART] Tx %s restaurada em pendentesCommit (%d trecho(s))", txID, len(e.Chaves))
	}
	pendentesCommit.mu.Unlock()
}
