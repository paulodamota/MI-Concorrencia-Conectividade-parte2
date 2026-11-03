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

// ---------------------------------------------------------------------------
// WAL do COORDENADOR — registra a decisão de 2PC para reservas de passageiro
// ---------------------------------------------------------------------------

// TipoWAL representa o estado de uma transação no log
type TipoWAL string

const (
	// WALPrepared: prepare OK, aguardando decisão do usuário (abortar em recovery)
	WALPrepared TipoWAL = "PREPARED"
	// WALCommitted: decisão tomada (ponto de não-retorno), reenviar commits em recovery
	WALCommitted TipoWAL = "COMMITTED"
	// WALDone: todos confirmaram ou foi abortado, entrada pode ser ignorada
	WALDone TipoWAL = "DONE"
)

func caminhoWAL() string {
	if cfg != nil && cfg.ServidorID > 0 {
		return fmt.Sprintf("dados/wal_reservas_srv%d.jsonl", cfg.ServidorID)
	}
	return "dados/wal_reservas.jsonl"
}

// EntradaWAL representa uma linha do log de transações
type EntradaWAL struct {
	Tipo       TipoWAL     `json:"tipo"`
	TxID       string      `json:"tx_id"`
	Passageiro string      `json:"passageiro,omitempty"`
	Servidores []bool      `json:"servidores,omitempty"`
	Viagem     *ViagemData `json:"viagem,omitempty"`
	Timestamp  time.Time   `json:"timestamp"`
}

var walMu sync.Mutex
var walArquivo *os.File

// InicializarWAL abre (ou cria) o arquivo de log em modo append.
// Deve ser chamado uma vez na inicialização, antes de RecuperarWAL.
func InicializarWAL() {
	// garante que o diretório existe (pode ser a primeira execução)
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

// GravarWAL acrescenta uma entrada ao log e faz fsync para garantir durabilidade.
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
	// fsync: garante que está em disco antes de continuar
	if err = walArquivo.Sync(); err != nil {
		log.Printf("[WAL] Erro no fsync do log: %v", err)
	}
}

// RecuperarWAL lê o log após um reinício e retoma transações incompletas.
// Deve ser chamado depois de InicializarWAL e antes de IniciarServidorREST.
func RecuperarWAL() {
	caminho := caminhoWAL()
	f, err := os.Open(caminho)
	if err != nil {
		if os.IsNotExist(err) {
			return // arquivo não existe ainda, sem recovery necessário
		}
		log.Printf("[WAL] Erro ao abrir log para recovery: %v", err)
		return
	}
	defer f.Close()

	// reconstrói o estado de cada transação lendo o log linha a linha
	// PREPARED é sobrescrito por COMMITTED; DONE remove a entrada
	estados := make(map[string]EntradaWAL)

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
			// mantém os dados do PREPARED (que tem servidores e viagem) mas atualiza o tipo
			if anterior, ok := estados[e.TxID]; ok {
				anterior.Tipo = WALCommitted
				estados[e.TxID] = anterior
			}
		case WALDone:
			delete(estados, e.TxID)
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
			// coordenador caiu enquanto esperava decisão do usuário → abortar
			log.Printf("[WAL RECOVERY] Abortando tx %s (estava em PREPARED — nenhuma decisão tomada)", e.TxID)
			ReservaAbort(e.TxID, e.Servidores)
			GravarWAL(EntradaWAL{Tipo: WALDone, TxID: e.TxID})

		case WALCommitted:
			// coordenador caiu após decidir COMMIT → reenviar commits aos participantes
			log.Printf("[WAL RECOVERY] Reenviando commits da tx %s (estava em COMMITTED)", e.TxID)
			entrada := e // cópia local para a goroutine
			go func() {
				// aguarda o servidor REST subir antes de enviar requisições HTTP
				time.Sleep(3 * time.Second)
				recuperarCommit(entrada)
			}()
		}
	}
}

// recuperarCommit reenvia os commits de uma transação que teve sua decisão gravada
// mas não foi concluída (coordenador caiu na fase 2).
func recuperarCommit(e EntradaWAL) {
	comitado := ReservaCommit(e.TxID, e.Passageiro, e.Servidores)

	if !comitado {
		log.Printf("[WAL RECOVERY] ATENÇÃO: tx %s ainda falhou após recovery — inconsistência possível!", e.TxID)
		// sem mais o que fazer; em produção, entraria em fila persistente de retry
		return
	}

	// garante que a reserva está registrada localmente
	// (pode já estar se o banco foi salvo após o commit, mas é idempotente)
	if e.Viagem != nil {
		ReservasPass.mu.Lock()
		if _, jaExiste := ReservasPass.Reservas[e.Passageiro][e.TxID]; !jaExiste {
			copia := *e.Viagem
			copia.ID = e.TxID
			if ReservasPass.Reservas[e.Passageiro] == nil {
				ReservasPass.Reservas[e.Passageiro] = InicializarReservasByUser()
			}
			ReservasPass.Reservas[e.Passageiro][e.TxID] = &copia
			log.Printf("[WAL RECOVERY] Reserva %s restaurada localmente para passageiro '%s'", e.TxID, e.Passageiro)
		}
		ReservasPass.mu.Unlock()
	}

	GravarWAL(EntradaWAL{Tipo: WALDone, TxID: e.TxID})
	log.Printf("[WAL RECOVERY] Tx %s concluída com sucesso no recovery.", e.TxID)
}

// ---------------------------------------------------------------------------
// WAL do PARTICIPANTE — registra o estado do prepare local para sobreviver a crashes
// ---------------------------------------------------------------------------

// Arquivo append-only com uma entrada JSON por linha.
// Permite que o participante reaplique commits recebidos após um reinício,
// mesmo que a trava em memória (transacoes) já tenha expirado.
func caminhoWALParticipante() string {
	if cfg != nil && cfg.ServidorID > 0 {
		return fmt.Sprintf("dados/wal_participante_srv%d.jsonl", cfg.ServidorID)
	}
	return "dados/wal_participante.jsonl"
}

// EntradaWALParticipante é uma linha no log do participante.
type EntradaWALParticipante struct {
	// "PREPARED" | "COMMITTED" | "ABORTED"
	Tipo      string         `json:"tipo"`
	TxID      string         `json:"tx_id"`
	// Chaves só é preenchido no PREPARED; é o que precisamos para decrementar Vagas
	Chaves    []TrechoChaves `json:"chaves,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

var (
	walPartMu      sync.Mutex
	walPartArquivo *os.File
)

// pendentesCommit guarda as chaves de trechos de transações que foram preparadas
// mas cuja decisão de commit ainda não chegou (o coordenador pode ter caído).
// Populado no recovery a partir do WAL do participante.
var pendentesCommit = struct {
	mu  sync.Mutex
	txs map[string][]TrechoChaves
}{txs: make(map[string][]TrechoChaves)}

// InicializarWALParticipante abre o arquivo de log do participante em modo append.
func InicializarWALParticipante() {
	caminho := caminhoWALParticipante()
	var err error
	walPartArquivo, err = os.OpenFile(caminho, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("[WAL PART] Falha ao abrir arquivo de log do participante: %v", err)
	}
	log.Println("[WAL PART] Log do participante inicializado:", caminho)
}

// GravarWALParticipante acrescenta uma entrada ao log do participante com fsync.
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

// RecuperarWALParticipante lê o log do participante na startup e popula
// pendentesCommit com as transações que foram preparadas mas não confirmadas.
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

	// mapa txID → EntradaWALParticipante com tipo atual
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
		case "COMMITTED", "ABORTED":
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

	// popula o mapa de pendentes para ser consultado pelo handlePostReservaCommit
	pendentesCommit.mu.Lock()
	for txID, e := range estados {
		pendentesCommit.txs[txID] = e.Chaves
		log.Printf("[WAL PART] Tx %s restaurada em pendentesCommit (%d trecho(s))", txID, len(e.Chaves))
	}
	pendentesCommit.mu.Unlock()
}
