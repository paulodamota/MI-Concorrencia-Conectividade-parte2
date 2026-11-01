package main

import (
	"log"
	"sync"

	. "vaijunto/shared"
)

type GrafoRotas struct {
	//mapa de string para lista de ponteiros de trecho
	//terei que iterar por toda a lista de trechos para achar(ou não) o destino
	//um map[str(orig)]map[str(destino)][]*Trecho acabaria com isso e daria O(1), mai mt trabalho
	Adjacencias map[string][]*Trecho
	mu          sync.RWMutex
}

func NovoGrafo() *GrafoRotas {
	return &GrafoRotas{
		Adjacencias: make(map[string][]*Trecho),
	}
}

// adiciona viagem de um motorista ao grafo
func (g *GrafoRotas) AdicionarViagem(user string, viagem *Viagem) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, t := range viagem.Trechos {
		g.Adjacencias[t.Origem] = append(g.Adjacencias[t.Origem], t)
	}
}

/*
BuscarRotas encontra todos os itinerários possíveis entre a origem e o destino
usando recursividade
*/
func (g *GrafoRotas) BuscarRotas(origem string, destino string, data string) []*ViagemData {
	var resultados []*ViagemData
	caminhoAtual := []TrechoData{}
	visitados := make(map[string]bool)

	g.dfsBusca(origem, destino, data, caminhoAtual, visitados, &resultados, []bool{false, false, false})
	return resultados
}

// estou mudando a busca para trabalhar com slice de trechodata em vez de slice de ponteiro de trecho
// busca em profundidade recursivo para achar caminhos compostos
func (g *GrafoRotas) dfsBusca(atual string, destino string, data string, caminho []TrechoData, visitados map[string]bool, resultados *[]*ViagemData, servidores []bool) {

	// caso base salva copia do caminho encontrado
	if atual == destino {
		caminhoCopia := make([]TrechoData, len(caminho))
		copy(caminhoCopia, caminho)
		*resultados = append(*resultados, &ViagemData{Trechos: caminhoCopia, Servidores: servidores})
		return
	}

	visitados[atual] = true
	defer delete(visitados, atual) //permite utilizar a cidade em outros caminhos

	g.mu.RLock()
	trechosDisponiveis := TrechoSliceToTrechoDataSlice(g.Adjacencias[atual])
	g.mu.RUnlock()

	trechosServidorB, err := ComandoParaServidor("/adjacencias/", atual, cfg.Vizinho(1))
	if err != nil {
		log.Printf("[GRAFO] Erro ao buscar adjacências do servidor %d: %v", cfg.Vizinho(1), err)
	} else {
		trechosDisponiveis = append(trechosDisponiveis, trechosServidorB.Trechos...)
	}

	trechosServidorC, err := ComandoParaServidor("/adjacencias/", atual, cfg.Vizinho(2))
	if err != nil {
		log.Printf("[GRAFO] Erro ao buscar adjacências do servidor %d: %v", cfg.Vizinho(2), err)
	} else {
		trechosDisponiveis = append(trechosDisponiveis, trechosServidorC.Trechos...)
	}

	for _, t := range trechosDisponiveis {
		// verifica se há vagas disponíveis
		// e se a cidade de destino foi visitada

		filtroData := (data == "" || data == t.Data)
		vagasDisponiveis := (t.Vagas > 0)

		if filtroData && vagasDisponiveis && !visitados[t.Destino] {
			// adiciona o trecho ao caminho
			caminho = append(caminho, t)

			//vai mapeando em quais servidores esse caminho está destribuido
			switch t.Servidor {
			case 1:
				servidores[0] = true
			case 2:
				servidores[1] = true
			case 3:
				servidores[2] = true
			}

			// continua a busca a partir da próxima cidade
			g.dfsBusca(t.Destino, destino, data, caminho, visitados, resultados, servidores)

			if len(*resultados) > 0 {
				return
			}

			// remove o ultimo trecho para testar outras opções
			caminho = caminho[:len(caminho)-1]
			switch t.Servidor {
			case 1:
				servidores[0] = false
			case 2:
				servidores[1] = false
			case 3:
				servidores[2] = false
			}
		}
	}
}

// devolve vagas aos trechos, quando o passageiro cancelar uma reserva ou recusar uma oferta de reserva
func (g *GrafoRotas) LiberarVagas(v *Viagem) {
	if len(v.Trechos) == 0 {
		return
	}

	trechosOrdenados := make([]*Trecho, len(v.Trechos))
	copy(trechosOrdenados, v.Trechos)
	for i := 0; i < len(trechosOrdenados)-1; i++ {
		for j := i + 1; j < len(trechosOrdenados); j++ {
			if trechosOrdenados[i].ID > trechosOrdenados[j].ID {
				trechosOrdenados[i], trechosOrdenados[j] = trechosOrdenados[j], trechosOrdenados[i]
			}
		}
	}

	for _, t := range trechosOrdenados {
		t.mu.Lock()
	}
	defer func() {
		for _, t := range trechosOrdenados {
			t.mu.Unlock()
		}
	}()

	for _, t := range trechosOrdenados {
		t.Vagas++
		//removendo o id da lista de id de viagens
		for i, vID := range t.ReservasAssociadasIDS {
			if vID.ReservaID == v.ID {
				t.ReservasAssociadasIDS[i] = t.ReservasAssociadasIDS[len(t.ReservasAssociadasIDS)-1]
				t.ReservasAssociadasIDS = t.ReservasAssociadasIDS[:len(t.ReservasAssociadasIDS)-1]
				break
			}
		}
	}
}

/*
remove a viagem cadastrada de um motorista, e retorna um slice de ids das viagens afetadas pela deleção
*/
func (g *GrafoRotas) RemoverViagemMotorista(viagem *Viagem) []ReservaChave {
	g.mu.Lock()
	defer g.mu.Unlock()

	var viagensAfetadas []ReservaChave

	for _, t := range viagem.Trechos {
		t.mu.Lock()
		viagensAfetadas = append(viagensAfetadas, t.ReservasAssociadasIDS...)
		t.mu.Unlock()

		trechosOrigem := g.Adjacencias[t.Origem]
		for i, trechoAdjacente := range trechosOrigem {
			if trechoAdjacente.ID == t.ID {
				g.Adjacencias[t.Origem] = append(trechosOrigem[:i], trechosOrigem[i+1:]...)
				break
			}
		}
	}

	return viagensAfetadas
}

//GRAFO funções relacionadas a api rest

func (g *GrafoRotas) GetAdjacencias(origem string) []TrechoData {
	g.mu.RLock()
	defer g.mu.RUnlock()

	trechos := g.Adjacencias[origem]
	return TrechoSliceToTrechoDataSlice(trechos)
}
