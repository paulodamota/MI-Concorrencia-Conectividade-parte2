package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	. "vaijunto/shared"
)

func main() {
	var servidorIP string

	if len(os.Args) > 1 {
		ipArg := os.Args[1]
		servidorIP = ipArg + ":6742" // Junta o IP digitado com a porta padrão
	}

	OverBooking(servidorIP)
	Concorrencia(servidorIP)
}

func OverBooking(ip string) {
	fmt.Println("=== INICIANDO TESTE DE OVERBOOKING (CONCORRÊNCIA DE RESERVAS) ===")

	motorista := "moto_teste"
	senhaMoto := "123"
	origem := "Centro"
	destino := "Praia"

	cadastrarMotoristaErota(ip, motorista, senhaMoto, origem, destino)

	const totalPassageiros = 50
	var wg sync.WaitGroup
	wg.Add(totalPassageiros)

	var sucessos int
	var falhas int
	var mu sync.Mutex

	for i := 1; i <= totalPassageiros; i++ {
		go func(id int) {
			defer wg.Done()

			nomePassageiro := fmt.Sprintf("pass_%d", id)

			conn, err := net.Dial("tcp", ip)
			if err != nil {
				fmt.Printf("[Passageiro %d] Erro ao conectar: %v\n", id, err)
				return
			}
			defer conn.Close()

			encoder := json.NewEncoder(conn)
			decoder := json.NewDecoder(conn)

			_ = encoder.Encode(Request{Acao: "CADASTRO_PASSAGEIRO", Nome: nomePassageiro, Senha: "123"})
			var res Resposta
			_ = decoder.Decode(&res)
			if res.Status != "SUCESSO" {
				_ = encoder.Encode(Request{Acao: "LOGIN_PASSAGEIRO", Nome: nomePassageiro, Senha: "123"})
				_ = decoder.Decode(&res)
			}

			_ = encoder.Encode(Request{Acao: "BUSCAR_E_RESERVAR", Nome: nomePassageiro, Origem: origem, Destino: destino})
			_ = decoder.Decode(&res)

			if res.Status == "SUCESSO" {
				_ = encoder.Encode(Request{Acao: "CONFIRMAR_ROTA", Nome: nomePassageiro})
				_ = decoder.Decode(&res)

				mu.Lock()
				if res.Status == "SUCESSO" {
					sucessos++
					fmt.Printf("[Passageiro %d] -> CONSEGUIU reservar a vaga!\n", id)
				} else {
					falhas++
					fmt.Printf("[Passageiro %d] -> Tentou confirmar, mas falhou: %s\n", id, res.Message)
				}
				mu.Unlock()
			} else {
				mu.Lock()
				falhas++
				fmt.Printf("[Passageiro %d] -> Não encontrou vagas disponíveis.\n", id)
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	fmt.Println("\n=== RESULTADO DO TESTE DE OVERBOOKING ===")
	fmt.Printf("Total de reservas bem-sucedidas: %d\n", sucessos)
	fmt.Printf("Total de reservas bloqueadas/falhas: %d\n", falhas)
	fmt.Println("==========================================")
}

func cadastrarMotoristaErota(ip string, motorista, senha, origem, destino string) {
	conn, err := net.Dial("tcp", ip)
	if err != nil {
		return
	}
	defer conn.Close()
	fmt.Print("oiiiiiiiiiii")

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	_ = encoder.Encode(Request{Acao: "CADASTRO_MOTORISTA", Nome: motorista, Senha: senha})
	var res Resposta
	_ = decoder.Decode(&res)
	if res.Status != "SUCESSO" {
		_ = encoder.Encode(Request{Acao: "LOGIN_MOTORISTA", Nome: motorista, Senha: senha})
		_ = decoder.Decode(&res)
	}

	trechos := []TrechoData{
		{Origem: origem, Destino: destino, Vagas: 1, Preco: 10.0},
	}

	_ = encoder.Encode(Request{Acao: "CADASTRAR_ROTA", Nome: motorista, Trechos: trechos})
	_ = decoder.Decode(&res)

	fmt.Printf("\n[%s] [%s]", res.Status, res.Message)
	fmt.Println("-> Motorista cadastrado e rota de 1 vaga criada com sucesso.")
}

func Concorrencia(ip string) {
	fmt.Println("=== INICIANDO TESTE DE CONCORRÊNCIA ===")

	const totalClientes = 50
	var wg sync.WaitGroup
	wg.Add(totalClientes)

	nomeUsuario := "teste_concorrente"
	senhaUsuario := "123456"

	cadastrarConta(ip, nomeUsuario, senhaUsuario)

	// Dispara várias goroutines simultâneas
	for i := 1; i <= totalClientes; i++ {
		go func(id int) {
			defer wg.Done()

			conn, err := net.Dial("tcp", ip)
			if err != nil {
				fmt.Printf("[Cliente %d] Erro ao conectar ao servidor: %v\n", id, err)
				return
			}
			defer conn.Close()

			encoder := json.NewEncoder(conn)
			decoder := json.NewDecoder(conn)

			// Envia requisição de login
			req := Request{
				Acao:  "LOGIN_PASSAGEIRO",
				Nome:  nomeUsuario,
				Senha: senhaUsuario,
			}

			if err := encoder.Encode(req); err != nil {
				fmt.Printf("[Cliente %d] Erro ao enviar requisição: %v\n", id, err)
				return
			}

			var res Resposta
			if err := decoder.Decode(&res); err != nil {
				fmt.Printf("[Cliente %d] Erro ao ler resposta: %v\n", id, err)
				return
			}

			fmt.Printf("[Cliente %d] Resposta recebida -> Status: %s | Mensagem: %s\n", id, res.Status, res.Message)
		}(i)
	}

	wg.Wait()
	fmt.Println("=== TESTE DE CONCORRÊNCIA FINALIZADO ===")
}

func cadastrarConta(ip string, nome, senha string) {
	conn, err := net.Dial("tcp", ip)

	if err != nil {
		return
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	_ = encoder.Encode(Request{
		Acao:  "CADASTRO_PASSAGEIRO",
		Nome:  nome,
		Senha: senha,
	})
	var res Resposta
	_ = decoder.Decode(&res)
}
