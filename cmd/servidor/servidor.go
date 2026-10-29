package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	. "vaijunto/shared"
)

// dados do servidor, usuarios viagens e grafo
var (
	//chave: nome (string)
	//dado:	 usua Usuario (ponteiro)
	Motoristas  sync.Map
	Passageiros sync.Map

	grafo = NovoGrafo()

	//estrutura com mutex e mapa de viagens
	ViagensMoto = NovoViagens()
	//estrutura com mutex e mapa de reservas
	ReservasPass = NovoReservas()

	//identificador desse servidor
	servidor int
	//endereço IP dos outros servidores, precisa ser substituido por um pra cada servidor!
	endereco1 string
	endereco2 string
	endereco3 string
	//portas desse servidor, [0]: porta TCP; [1]: porta HTTP
	portas []string

	transacoes = InicializarTransacoes()

	//urls dos urlsDosServidores
	urlsDosServidores []string
)

func GetPorta(serv int) []string {
	switch serv {
	//primeira porta para socket tcp (comunicação com clientes usuarios)
	//segunda porta para http
	case 1: //sergipe
		return []string{"2800", "2828"}
	case 2: //bahia
		return []string{"2900", "2929"}
	case 3: //minas gerais
		return []string{"3100", "3131"}
	default:
		return nil
	}
}

var tabelaVizinhos = [][]int{
	{},        // Índice 0 (ignorado)
	{0, 2, 3}, // Servidor 1 -> vizinho 1 retorna 2, vizinho 2 retorna 3
	{0, 3, 1}, // Servidor 2 -> vizinho 1 retorna 3, vizinho 2 retorna 1
	{0, 2, 1}, // Servidor 3 -> vizinho 1 retorna 2, vizinho 2 retorna 1
}

func GetVizinho(vizinho int) int {
	if servidor <= 0 || servidor >= len(tabelaVizinhos) {
		return 0
	}
	if vizinho <= 0 || vizinho >= len(tabelaVizinhos[servidor]) {
		return 0
	}

	return tabelaVizinhos[servidor][vizinho]
}

func GetURLHTTP(serv int) string {
	switch serv {
	case 1: //sergipe
		return "http://" + endereco1 + ":" + GetPorta(serv)[1]
	case 2: //bahia
		return "http://" + endereco2 + ":" + GetPorta(serv)[1]
	case 3: //minas gerais
		return "http://" + endereco3 + ":" + GetPorta(serv)[1]
	default:
		return ""
	}
}

func main() {
	var err error
	servidor, err = strconv.Atoi(strings.TrimSpace(os.Getenv("servidor")))
	if err != nil || servidor == 0 {
		fmt.Print("\nPorta de servidor não especificada!\n")
		return
	}

	endereco1 = os.Getenv("endereco1")
	if endereco1 == "" {
		fmt.Print("\nEndereco ip 1 não especificado!\n")
		return
	}
	endereco2 = os.Getenv("endereco2")
	if endereco2 == "" {
		fmt.Print("\nEndereco ip 2 não especificado!\n")
		return
	}
	endereco3 = os.Getenv("endereco3")
	if endereco3 == "" {
		fmt.Print("\nEndereco ip 3 não especificado!\n")
		return
	}

	urlsDosServidores = []string{
		GetURLHTTP(1),
		GetURLHTTP(2),
		GetURLHTTP(3),
	}

	portas = GetPorta(servidor)
	if portas == nil {
		fmt.Print("\nPorta inválida!\n")
		return
	}

	//carrega os dados salvos
	Carregar()

	//salva dados de tempos em tempos
	go func() {
		for {
			time.Sleep(10 * time.Second)
			Salvar()
			//fmt.Printf("\t[BackUp!]")
		}
	}()

	//configura para salvar os logs em arquivos
	ConfigurarLogs()

	IniciarServidorREST()
	//configurando a conexão de usuário
	port := "0.0.0.0:" + portas[0]
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Erro ao iniciar servidor TCP: %v", err)
	}
	//fechar a conexão ao final
	defer listener.Close()

	log.Printf("=== SERVIDOR INICIADO ===\n")

	//expera por clientes
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Erro ao aceitar conexão: %v", err)
			continue
		}
		go handleConnection(conn)
	}
}

/*
Lida com conexões de cliente,
direcionando a funções especificas para motorista ou passageiro após login/cadastro
*/
func handleConnection(conn net.Conn) {
	defer conn.Close()

	//usado para traduzir a mensagem em formato json para envio dos pacotes
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var conectado bool = true

	//loop enquanto estiver conectado
	for conectado {
		var req Request
		err := decoder.Decode(&req)
		if err != nil {
			log.Printf("Conexão encerrada no menu principal: %v", err)
			break
		}

		var userHandler func()
		var res Resposta

		switch req.Acao {

		//++++++++++++++++++MOTORISTA+++++++++++++++++++
		case "CADASTRO_MOTORISTA":
			res = Cadastrar(req, &Motoristas)

			if res.Status == "SUCESSO" {

				ViagensMoto.mu.Lock()
				ViagensMoto.Viagens[req.Nome] = InicializarViagensByUser()
				ViagensMoto.mu.Unlock()

				log.Printf("Novo Motorista \"%s\" cadastrado com senha '%s'", req.Nome, req.Senha)
				userHandler = func() { MotoristaConn(encoder, decoder, req.Nome) }
			}

		case "LOGIN_MOTORISTA":
			res = Login(req, &Motoristas)

			if res.Status == "SUCESSO" {
				log.Printf("Motorista \"%s\" fez login", req.Nome)
				userHandler = func() { MotoristaConn(encoder, decoder, req.Nome) }
			}

		//+++++++++++++++++PASSAGEIRO++++++++++++++++++++
		case "CADASTRO_PASSAGEIRO":
			res = Cadastrar(req, &Passageiros)

			if res.Status == "SUCESSO" {

				ReservasPass.mu.Lock()
				ReservasPass.Reservas[req.Nome] = InicializarReservasByUser()
				ReservasPass.mu.Unlock()

				log.Printf("Novo Passageiro \"%s\" cadastrado com senha '%s'", req.Nome, req.Senha)
				userHandler = func() { PassageiroConn(encoder, decoder, req.Nome) }
			}

		case "LOGIN_PASSAGEIRO":
			res = Login(req, &Passageiros)

			if res.Status == "SUCESSO" {
				log.Printf("Passageiro \"%s\" fez login", req.Nome)
				userHandler = func() { PassageiroConn(encoder, decoder, req.Nome) }
			}

		//++++++++++++++++++++++++++++++++++++
		case "DESCONECTAR":
			conectado = false
			res = Resposta{
				Status:  "SUCESSO",
				Message: "A conexão com o servidor será encerrada.",
			}
		default:
			res = Resposta{
				Status:  "ERROR",
				Message: fmt.Sprintf("Acao não reconhecida: '%s' . Operação Cancelada", req.Acao),
			}
		}

		//
		if err := encoder.Encode(res); err != nil {
			log.Printf("Erro ao enviar resposta: %v", err)
			break
		}

		if userHandler != nil {
			userHandler()
		}

	}
}
