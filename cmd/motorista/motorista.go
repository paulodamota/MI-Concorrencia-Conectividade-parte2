package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"time"
	. "vaijunto/shared"
)

func main() {
	endereco := os.Getenv("endereco")
	if endereco == "" {
		fmt.Print("\nCade o endereco?!?!\n")
		return
	}

	conn, err := net.Dial("tcp", endereco)
	if err != nil {
		log.Fatalf("Erro ao conectar ao servidor: %v", err)
	}
	defer conn.Close()

	scanner := bufio.NewScanner(os.Stdin)
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	fmt.Println("=== CLIENTE MOTORISTA ===")

	var conectado bool = true
	for conectado {
		fmt.Println("\nEscolha uma opção: ")
		fmt.Println("1. Login")
		fmt.Println("2. Nova conta")
		fmt.Println("3. Encerrar programa")

		scanner.Scan()
		input := scanner.Text()

		switch input {
		// case 1 e 2, diferem em apenas duas linha, como posso melhorar isso?
		case "1": //login
			{
				fmt.Print("Digite o nome de Usuário:")
				scanner.Scan()
				nome := scanner.Text()

				fmt.Print("Digite a senha:")
				scanner.Scan()
				senha := scanner.Text()

				var req Request = Request{
					Acao:  "LOGIN_MOTORISTA",
					Nome:  nome,
					Senha: senha,
				}

				//escreve request na stream da conexão tcp
				err = encoder.Encode(req)
				if err != nil {
					fmt.Println("Erro ao enviar mensagem para o servidor!")
					fmt.Println("Programa será encerrado")
					os.Exit(1)
				}

				//le resposta da conexão
				var resp Resposta
				if err := decoder.Decode(&resp); err != nil {
					log.Printf("Erro ao ler resposta do servidor: %v", err)
					fmt.Println("Programa será encerrado")
					os.Exit(1)
				}

				//exibe resposta para o cliente
				fmt.Printf("\nServidor diz: [%s] %s\n", resp.Status, resp.Message)

				if resp.Status == "SUCESSO" {
					LoggedIn(req.Nome, encoder, decoder, scanner)
				}
			}
		case "2": //cadastro
			{
				fmt.Println("Digite o nome de Usuário:")
				scanner.Scan()
				nome := scanner.Text()

				fmt.Println("Digite a senha:")
				scanner.Scan()
				senha := scanner.Text()

				var req Request = Request{
					Acao:  "CADASTRO_MOTORISTA",
					Nome:  nome,
					Senha: senha,
				}

				err = encoder.Encode(req)
				if err != nil {
					fmt.Println("Erro ao enviar mensagem para o servidor!")
					fmt.Println("Programa será encerrado")
					os.Exit(1)
				}

				var resp Resposta
				if err := decoder.Decode(&resp); err != nil {
					log.Printf("Erro ao ler resposta do servidor: %v", err)
					fmt.Println("Programa será encerrado")
					os.Exit(1)
				}

				fmt.Printf("\nServidor diz: [%s] %s\n", resp.Status, resp.Message)

				if resp.Status == "SUCESSO" {
					fmt.Println("\nFazendo login Automático após cadastro...")
					LoggedIn(req.Nome, encoder, decoder, scanner)
				}
			}
		case "3":
			var req Request = Request{
				Acao: "DESCONECTAR",
			}

			err = encoder.Encode(req)
			if err != nil {
				fmt.Println("Erro ao enviar mensagem para o servidor!")
				fmt.Println("Programa será encerrado")
				os.Exit(1)
			}

			var resp Resposta
			if err := decoder.Decode(&resp); err != nil {
				log.Printf("Erro ao ler resposta do servidor: %v", err)
				fmt.Println("Programa será encerrado")
				os.Exit(1)
			}

			fmt.Printf("\nServidor diz: [%s] %s\n", resp.Status, resp.Message)

			fmt.Print("\nPrograma cliente encerrado.")

			conectado = false

		default:
			fmt.Print("\nOpção não reconhecido.")
		}

		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "Erro ao ler a entrada:", err)
		}

	}
}

func LoggedIn(user string, encoder *json.Encoder, decoder *json.Decoder, scanner *bufio.Scanner) {

	for {
		fmt.Println("\nEscolha uma opção: ")
		fmt.Println("1. Cadastrar Nova Rota")
		fmt.Println("2. Remover Uma Rota")
		fmt.Println("3. Listar Rotas")
		fmt.Println("4. Log Out")

		scanner.Scan()
		input := scanner.Text()

		switch input {
		case "1": //cadastrar rota
			{
				ImprimirCidades()

				paradas, cancelado := ConstruirRota(scanner)
				if cancelado {
					fmt.Println("Operação cancelada")
					continue
				}

				trechos, cancelado := DefinirVagasTrechos(scanner, paradas, user)
				if cancelado {
					fmt.Println("Operação cancelada")
					continue
				}

				var req = Request{
					Acao:    "CADASTRAR_ROTA",
					Origem:  paradas[0],
					Destino: paradas[len(paradas)-1],
					Trechos: trechos,
				}

				err := encoder.Encode(req)
				if err != nil {
					fmt.Println("Erro ao enviar mensagem para o servidor!")
					fmt.Println("Programa será encerrado")
					os.Exit(1)
				}

				var resp Resposta
				if err := decoder.Decode(&resp); err != nil {
					log.Printf("Erro ao ler resposta do servidor: %v", err)
					fmt.Println("Programa será encerrado")
					os.Exit(1)
				}

				fmt.Printf("\n[%s] %s\n", resp.Status, resp.Message)

			}

		case "2": //remover rota
			{
				var req = Request{
					Acao: "LISTAR_ROTAS",
				}

				err := encoder.Encode(req)
				if err != nil {
					fmt.Println("Erro ao enviar mensagem para o servidor!")
					fmt.Println("Programa será encerrado")
					os.Exit(1)
				}

				var res Resposta
				_ = decoder.Decode(&res)

				fmt.Printf("\nServidor diz: [%s] %s\n", res.Status, res.Message)

				ImprimirViagens(res.Viagens)

				//+++++++tratamento para a leitura do indice+++++++
				fmt.Println("Digite 'c' para cancelar.")
				fmt.Println("Digite o Indice da reserva que deseja remover:")
				num, cancelado := LerInt(scanner)
				if cancelado {
					fmt.Println("Operação cancelada")
					continue
				}
				indice := num - 1
				if indice < 0 || indice >= len(res.Viagens) {
					fmt.Println("Indice de viagem invalido.")
					fmt.Println("Operação encerrada.")
					continue
				}
				//++++++++++++++++++++++++++++++++++++++++++++++++++

				remover := res.Viagens[indice].ID

				req = Request{
					Acao: "REMOVER_VIAGEM",
					ID:   remover,
				}

				err = encoder.Encode(req)
				if err != nil {
					fmt.Println("Erro ao enviar mensagem para o servidor!")
					fmt.Println("Programa será encerrado")
					os.Exit(1)
				}
				_ = decoder.Decode(&res)
				fmt.Printf("Servidor diz: \n[%s] %s\n", res.Status, res.Message)
				fmt.Printf("\nViagem de ID %s removida", remover)

			}
		case "3": //listar rotas
			{
				var req = Request{
					Acao: "LISTAR_ROTAS",
				}

				err := encoder.Encode(req)
				if err != nil {
					fmt.Println("Erro ao enviar mensagem para o servidor!")
					fmt.Println("Programa será encerrado")
					os.Exit(1)
				}

				var resp Resposta
				if err := decoder.Decode(&resp); err != nil {
					log.Printf("Erro ao ler resposta do servidor: %v", err)
					fmt.Println("Programa será encerrado")
					os.Exit(1)
				}

				fmt.Printf("\nServidor diz: [%s] %s\n", resp.Status, resp.Message)

				ImprimirViagens(resp.Viagens)
			}
		case "4": //logout
			{
				var req = Request{
					Acao: "LOG_OUT",
				}

				err := encoder.Encode(req)
				if err != nil {
					fmt.Println("Erro ao enviar mensagem para o servidor!")
					fmt.Println("Programa será encerrado")
					os.Exit(1)
				}

				var resp Resposta
				if err := decoder.Decode(&resp); err != nil {
					log.Printf("Erro ao ler resposta do servidor: %v", err)
					fmt.Println("Programa será encerrado")
					os.Exit(1)
				}

				fmt.Printf("\nServidor diz: [%s] %s\n", resp.Status, resp.Message)

				return
			}
		default:
		}
	}
}

func ConstruirRota(scanner *bufio.Scanner) ([]string, bool) {
	var paradas []string

	fmt.Println("\n--- Cadastro de Rota ---")
	fmt.Println("Digite '-1' para finalizar ou 'c' para cancelar.")

	fmt.Println("Informe o código da cidade (Origem->Parada->Destino) (minimo de 2 cidades para finalizar) ")
	for {

		fmt.Printf("\nInforme o código da cidade #%d: ", len(paradas)+1)

		num, cancelado := LerInt(scanner)

		if cancelado {
			return nil, true
		}

		if num == -1 {
			if len(paradas) < 2 {
				fmt.Println("Erro: Adicione pelo menos duas cidades.")
				continue
			}
			break
		}

		cidade := Cidade(num).GetCidade()
		if cidade == "INVALIDA" {
			fmt.Println("Erro: O código informado não corresponde a nenhuma cidade válida.")
			continue
		}

		if (len(paradas) > 0) && (cidade == paradas[len(paradas)-1]) {
			fmt.Println("Erro: O código informado não pode ser igual ao anterior.")
			continue
		}

		paradas = append(paradas, cidade)
		fmt.Printf("Adicionado: %s\n", cidade)
	}

	return paradas, false
}

func DefinirVagasTrechos(scanner *bufio.Scanner, paradas []string, user string) ([]TrechoData, bool) {
	var trechos []TrechoData

	fmt.Println("\n--- Definição de Vagas por Trecho ---")

	fmt.Println("\nQual o preco por vaga?: ")
	preco, cancelado := LerFloat(scanner)
	if cancelado {
		return nil, true
	}

	for i := 0; i < len(paradas)-1; i++ {
		origem := paradas[i]
		destino := paradas[i+1]

		fmt.Printf("Quantas vagas estarão disponíveis para o trecho de %s a %s?: ", origem, destino)

		vagas, cancelado := LerInt(scanner)
		if cancelado {
			return nil, true
		}

		fmt.Printf("Qual a data da viagem? (Padrão dd-mm-yyyy) ")
		scanner.Scan()
		data := scanner.Text()
		_, err := time.Parse("02-01-2006", data)
		for err != nil {
			fmt.Print("Formata de data invalido!!, Siga o padrão: dd-mm-yyyy")
			scanner.Scan()
			data = scanner.Text()
			_, err = time.Parse("02-01-2006", data)
		}

		fmt.Printf("Qual o horario da viagem?: ")
		scanner.Scan()
		horario := scanner.Text()

		trechos = append(trechos, TrechoData{
			Motorista: user,
			Origem:    origem,
			Destino:   destino,
			Vagas:     vagas,
			Preco:     preco,
			Data:      data,
			Horario:   horario,
		})
	}

	return trechos, false
}
