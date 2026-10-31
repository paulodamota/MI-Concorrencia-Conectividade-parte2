package shared

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

type Request struct {
	Acao string `json:"acao"`
	ID   string `json:"id,omitempty"`

	Nome  string `json:"nome,omitempty"`
	Senha string `json:"senha,omitempty"`

	Origem  string       `json:"origem,omitempty"`
	Destino string       `json:"destino,omitempty"`
	Data    string       `json:"Data,omitempty"`
	Trechos []TrechoData `json:"trechos,omitempty"` //quando o motorista vai montar a rota ele envia os trechos
}

type Resposta struct {
	Status  string `json:"status"`
	Message string `json:"message"`

	Notificacoes []string     `json:"notificacoes,omitempty"`
	Viagens      []ViagemData `json:"trechos,omitempty"`
}

/*
Structs para construir pacotes de resposta e request
*/
type TrechoData struct {
	ID        string  `json:"id"`
	ViagemID  string  `json:"viagemid"`
	Motorista string  `json:"motorista"`
	Origem    string  `json:"origem"`
	Destino   string  `json:"destino"`
	Vagas     int     `json:"vagas"`
	Preco     float64 `json:"preco"`
	Data      string  `json:"data"`
	Horario   string  `json:"horario"`
	Servidor  int     `json:"servidor"`

	ViagensAssociadasIDS []ReservaChave `json:"viagensAssociadasids"`
}

/*
usado no pacote e nos trechos para rastrear mais facilmente usarios afetados por deleção de viagem de um motorista
*/
type ReservaChave struct {
	ReservaID string `json:"id"`
	User      string `json:"user"`
	Servidor  int    `json:"servidor"`
}

type ViagemData struct {
	ID         string       `json:"id"`
	Dono       string       `json:"dono"`
	Trechos    []TrechoData `json:"trechos"`
	Servidores []bool       `json:"servidores"` //campo temporario usado para rastrear quais servidores estão envolvidos na reservar durante a busca no grafo
}

func GetCidade(estado string, numero int) string {
	switch estado {
	case "BA":
		switch numero {
		case 1:
			return "Feira de Santana"
		case 2:
			return "Salvador"
		case 3:
			return "Simões Filho"
		case 4:
			return "Xique-Xique"
		case 5:
			return "Vitória da Conquista"
		case 6:
			return "Camaçari"
		case 7:
			return "Juazeiro"
		case 8:
			return "Itabuna"
		case 9:
			return "Ilhéus"
		case 10:
			return "Porto Seguro"
		default:
			return "INVALIDA"
		}

	case "MG":
		switch numero {
		case 1:
			return "Belo Horizonte"
		case 2:
			return "Uberlândia"
		case 3:
			return "Juiz de Fora"
		case 4:
			return "Ouro Preto"
		case 5:
			return "Tiradentes"
		case 6:
			return "Diamantina"
		case 7:
			return "Caldas"
		case 8:
			return "Capitólio"
		case 9:
			return "Montes Claros"
		case 10:
			return "Ipatinga"
		default:
			return "INVALIDA"
		}

	case "SE":
		switch numero {
		case 1:
			return "Aracaju"
		case 2:
			return "Nossa Senhora do Socorro"
		case 3:
			return "Itabaiana"
		case 4:
			return "Lagarto"
		case 5:
			return "São Cristóvão"
		case 6:
			return "Laranjeiras"
		case 7:
			return "Canindé de São Francisco"
		case 8:
			return "Estância"
		case 9:
			return "Propriá"
		case 10:
			return "Tobias Barreto"
		default:
			return "INVALIDA"
		}

	default:
		return "INVALIDA"
	}
}

func ImprimirCidades() {
	fmt.Printf("%-6s | %-25s | %-25s | %s\n", "NÚMERO", "BA - Bahia", "MG - Minas Gerais", "SE - Sergipe")
	fmt.Println("-------------------------------------------------------------------------------")

	for i := 1; ; i++ {
		ba := GetCidade("BA", i)
		mg := GetCidade("MG", i)
		se := GetCidade("SE", i)
		if ba == "INVALIDA" && mg == "INVALIDA" && se == "INVALIDA" {
			break
		}
		if ba == "INVALIDA" {
			ba = ""
		}
		if mg == "INVALIDA" {
			mg = ""
		}
		if se == "INVALIDA" {
			se = ""
		}
		fmt.Printf("%-6d | %-25s | %-25s | %s\n", i, ba, mg, se)
	}
}

func ImprimirViagens(viagens []ViagemData) {
	fmt.Println("==================== RESUMO DE VIAGENS ====================")

	for h, viagem := range viagens {
		fmt.Printf("Viagem #%d:\n", h+1)
		fmt.Printf("\tMotorista : %s\n", viagem.Dono)
		fmt.Printf("\tID da viagem : %s\n", viagem.ID)
		fmt.Printf("\n\tTotal de Trechos: %d\n", len(viagem.Trechos))

		if len(viagem.Trechos) > 0 {
			fmt.Println("\tDetalhes dos Trechos:")
			for i, trecho := range viagem.Trechos {
				fmt.Printf("\n")
				fmt.Printf("\tTrecho #%d:", i+1)
				fmt.Printf("\t%s -> %s\n", trecho.Origem, trecho.Destino)
				fmt.Printf("\t\tID: %s\n", trecho.ID)
				fmt.Printf("\t\tData: %s, Horario:%s \n", trecho.Data, trecho.Horario)
				fmt.Printf("\t\tVagas: %d | Preço: R$ %.2f\n\n", trecho.Vagas, trecho.Preco)
				fmt.Printf("\t\tPassageiros desse trecho (%d):\n", len(trecho.ViagensAssociadasIDS))
				for j, t := range trecho.ViagensAssociadasIDS {
					fmt.Printf("\t\t\t#%d: '%s'\n", j+1, t.User)
				}
			}
		} else {
			fmt.Println("  Nenhum trecho cadastrado para esta viagem.")
		}

		fmt.Println("-----------------------------------------------------------")
	}
	fmt.Println("===========================================================")

}

func ImprimirReservas(viagens []ViagemData) {
	fmt.Println("======================== RESUMO DE RESERVAS ========================")

	for h, viagem := range viagens {
		fmt.Printf("Reserva #%d:\n", h+1)
		fmt.Printf("\tDono : %s\n", viagem.Dono)
		fmt.Printf("\tID da reserva: %s\n", viagem.ID)
		fmt.Printf("\n\tTotal de Trechos: %d\n", len(viagem.Trechos))

		if len(viagem.Trechos) > 0 {
			fmt.Println("\tDetalhes dos Trechos:")
			for i, trecho := range viagem.Trechos {
				fmt.Printf("\n")
				fmt.Printf("\tTrecho #%d:\n", i)
				fmt.Printf("\t\tID do trecho: %s\n", trecho.ID)
				fmt.Printf("\t\t%s -> %s\n", trecho.Origem, trecho.Destino)
				fmt.Printf("\t\tData: %s, Horario:%s \n", trecho.Data, trecho.Horario)
				fmt.Printf("\t\tPreço Por Vaga: R$ %.2f\n", trecho.Preco)
				fmt.Printf("\t\tMotorista desse trecho: %s\n", trecho.Motorista)
			}
		} else {
			fmt.Println("  Nenhum trecho cadastrado para esta reserva.")
		}

		fmt.Println("------------------------------------------------------------------")
	}
	fmt.Println("==================================================================")
}

func ImprimirReservasEscolha(viagens []ViagemData) {
	fmt.Println("======================== ROTA  ENCONTRADA ========================")

	for h, viagem := range viagens {
		fmt.Printf("Reserva #%d:\n", h+1)
		fmt.Printf("\tDono : %s\n", viagem.Dono)
		fmt.Printf("\tID da reserva: %s\n", viagem.ID)
		fmt.Printf("\n\tTotal de Trechos: %d\n", len(viagem.Trechos))

		if len(viagem.Trechos) > 0 {
			fmt.Println("\tDetalhes dos Trechos:")
			for i, trecho := range viagem.Trechos {
				fmt.Printf("\n")
				fmt.Printf("\tTrecho #%d:\n", i)
				fmt.Printf("\t\tID do trecho: %s\n", trecho.ID)
				fmt.Printf("\t\t%s -> %s\n", trecho.Origem, trecho.Destino)
				fmt.Printf("\t\tData: %s, Horario:%s \n", trecho.Data, trecho.Horario)
				fmt.Printf("\t\tVagas: %d | Preço: R$ %.2f\n", trecho.Vagas, trecho.Preco)
				fmt.Printf("\t\tMotorista desse trecho: %s\n", trecho.Motorista)
			}
		} else {
			fmt.Println("  Nenhum trecho cadastrado para esta reserva.")
		}

		fmt.Println("------------------------------------------------------------------")
	}
	fmt.Println("==================================================================")
}

// recebe uma string como "BA-1" ou "MG, 4" e retorna o estado e o número da cidade.
func LerCidade(scanner *bufio.Scanner) (string, int) {
	for {
		// Limpa espaços e padroniza para maiúsculo
		fmt.Print("> ")
		if !scanner.Scan() {
			return "", -1
		}

		input := strings.TrimSpace(strings.ToUpper(scanner.Text()))

		if strings.ToLower(input) == "c" {
			return "", -1
		}

		if strings.ToLower(input) == "f" {
			return "", -2
		}

		//validação
		parts := strings.Split(input, "#")
		if len(parts) != 2 {
			fmt.Println("Erro: formato inválido, use ESTADO#NUMERO (ex: BA#1)")
			continue
		}

		estado := strings.TrimSpace(parts[0])
		numStr := strings.TrimSpace(parts[1])

		numero, err := strconv.Atoi(numStr)
		if err != nil {
			fmt.Println("Erro: O valor digitado para cidade não é um número")
			continue
		}
		if GetCidade(estado, numero) == "INVALIDA" {
			fmt.Println("Erro: Estado ou cidade informada não é válida")
			continue
		}

		if estado != "BA" && estado != "MG" && estado != "SE" {
			fmt.Println("Erro: O estado informado não é válido")
			continue
		}

		return estado, numero
	}
}

func LerInt(scanner *bufio.Scanner) (int, bool) {
	for {
		if !scanner.Scan() {
			return 0, true
		}

		texto := strings.TrimSpace(scanner.Text())

		if strings.ToLower(texto) == "c" {
			return -1, true
		}

		num, err := strconv.Atoi(texto)
		if err != nil {
			fmt.Println("Erro: O valor digitado não é um número")
			continue
		}

		return num, false
	}
}

func LerFloat(scanner *bufio.Scanner) (float64, bool) {
	for {
		if !scanner.Scan() {
			return 0, true
		}

		texto := strings.TrimSpace(scanner.Text())

		if strings.ToLower(texto) == "c" {
			return -1, true
		}

		texto = strings.ReplaceAll(texto, ",", ".")

		num, err := strconv.ParseFloat(texto, 64)
		if err != nil {
			fmt.Println("Erro: O valor digitado não é um número válido")
			continue
		}

		return num, false
	}
}

func ImprimirNotificacoes(notificacoes []string) {
	fmt.Println("\n=============+++++++++++++ NOTIFICAÇÕES +++++++++++++=============")

	if len(notificacoes) == 0 {
		fmt.Println(" Você não tem nenhuma notificação nova.")
		fmt.Println("=============+++++++++++++==============+++++++++++++=============")
		return
	}

	for i, msg := range notificacoes {
		fmt.Printf(" [#%d] %s\n", i+1, msg)
	}

	fmt.Println("=============+++++++++++++==============+++++++++++++=============")
}
