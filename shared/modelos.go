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
	ViagemID  string  `json:"viagemid"`
	Motorista string  `json:"motorista"`
	ID        string  `json:"id"`
	Origem    string  `json:"origem"`
	Destino   string  `json:"destino"`
	Vagas     int     `json:"vagas"`
	Preco     float64 `json:"preco"`
	Data      string  `json:"data"`
	Horario   string  `json:"horario"`

	ViagensAssociadasIDS []Par `json:"par"`
	//horario e data depois
}

/*
usado no pacote e nos trechos para rastrear mais facilmente usarios afetados por deleção de viagem de um motorista
*/
type Par struct {
	ID   string
	User string
}

type ViagemData struct {
	ID      string
	Dono    string
	Trechos []TrechoData
}

/*
enum para cidades permitidas
*/
type Cidade int

const (
	Feira Cidade = iota + 1
	Salvador
	SimoesFilho
	Xique
	Conquista
	Camacari
	Juazeiro
	Itabuna
	Ilheus
	PortoSeguro
	Barreiras
)

func (c Cidade) GetCidade() string {
	switch c {
	case Feira:
		return "Feira de Santana"
	case Salvador:
		return "Salvador"
	case SimoesFilho:
		return "Simões Filho"
	case Xique:
		return "Xique-Xique"
	case Conquista:
		return "Vitória da Conquista"
	case Camacari:
		return "Camaçari"
	case Juazeiro:
		return "Juazeiro"
	case Itabuna:
		return "Itabuna"
	case Ilheus:
		return "Ilhéus"
	case PortoSeguro:
		return "Porto Seguro"
	case Barreiras:
		return "Barreiras"
	default:
		return "INVALIDA"
	}
}

func ImprimirCidades() {
	fmt.Printf("%-6s | %s\n", "NÚMERO", "NOME")
	fmt.Println("--------------------------------")

	for i := 1; ; i++ {
		c := Cidade(i)
		if c.GetCidade() == "INVALIDA" {
			break
		}
		fmt.Printf("%-6d | %s\n", int(c), c.GetCidade())
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
	fmt.Println("================ RESUMO DE RESERVAS ================")

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

		fmt.Println("---------------------------------------------------")
	}
	fmt.Println("===================================================")
}

func ImprimirReservasEscolha(viagens []ViagemData) {
	fmt.Println("================ RESUMO DE RESERVAS ================")

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
				fmt.Printf("\t\tVagas: %d | Preço: R$ %.2f\n", trecho.Vagas+1, trecho.Preco)
				fmt.Printf("\t\tMotorista desse trecho: %s\n", trecho.Motorista)
			}
		} else {
			fmt.Println("  Nenhum trecho cadastrado para esta reserva.")
		}

		fmt.Println("---------------------------------------------------")
	}
	fmt.Println("===================================================")
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
