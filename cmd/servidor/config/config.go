// configurações do servidor
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

/*
tempos de espera do protocolo http e 2pc
*/
const (
	TimeoutHTTP     = 200 * time.Millisecond
	EsperaMaxima    = 130 * time.Millisecond
	JanelaOrdenacao = 50 * time.Millisecond
)

// portas tcp [0] e portas http [1]
var portas = map[int][2]string{
	1: {"2800", "2828"}, // sergipe
	2: {"2900", "2929"}, // bahia
	3: {"3100", "3131"}, // minas gerais
}

// tabela dos ids dos vizinhos
var vizinhos = map[int][2]int{
	1: {2, 3},
	2: {3, 1},
	3: {2, 1},
}

// configurações e dados do servidor em execução
type Config struct {
	ServidorID int
	PortaTCP   string
	PortaHTTP  string

	urls      []string
	enderecos [3]string
}

// lê as variáveis de ambiente id deste servidor, endereco1, endereco2, endereco3 (endereços desse e dos outros servidores)
func LerConfig() (*Config, error) {
	id, err := strconv.Atoi(strings.TrimSpace(os.Getenv("servidor")))
	if err != nil || id < 1 || id > 3 {
		return nil, fmt.Errorf("variável 'servidor' inválida ou não especificada (use 1, 2 ou 3)")
	}

	c := &Config{
		ServidorID: id,
		PortaTCP:   portas[id][0],
		PortaHTTP:  portas[id][1],
	}

	for i := 0; i < 3; i++ {
		nome := fmt.Sprintf("endereco%d", i+1)
		c.enderecos[i] = os.Getenv(nome)
		if c.enderecos[i] == "" {
			return nil, fmt.Errorf("variável '%s' não especificada", nome)
		}
	}

	c.urls = []string{c.URLHTTP(1), c.URLHTTP(2), c.URLHTTP(3)}
	return c, nil
}

// devolve a URL HTTP do servidor ou "" se for inválido
func (c *Config) URLHTTP(serv int) string {
	if serv < 1 || serv > 3 {
		return ""
	}
	return "http://" + c.enderecos[serv-1] + ":" + portas[serv][1]
}

// devolve as urls, vai que eu altero sem querer sadesgraca
func (c *Config) GetUrls() []string { return c.urls }

// devolve o id do vizinho deste servidor ou 0 se inválido
func (c *Config) Vizinho(n int) int {
	if n < 1 || n > 2 {
		return 0
	}
	return vizinhos[c.ServidorID][n-1]
}
