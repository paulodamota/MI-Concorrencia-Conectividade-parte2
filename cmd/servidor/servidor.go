package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
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

	ViagensMoto = NovoViagens()
	ViagensPass = NovoViagens()
)

func main() {

	//carrega os dados salvos
	Carregar()

	//salva dados de tempos em tempos
	go func() {
		for {
			time.Sleep(10 * time.Second)
			Salvar()
			fmt.Printf("\t[BackUp!]")
		}
	}()

	//configura para salvar os logs em arquivos
	ConfigurarLogs()

	//configurando a conexão
	port := "0.0.0.0:6742"
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
				ViagensPass.Viagens[req.Nome] = InicializarViagensByUser()
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
