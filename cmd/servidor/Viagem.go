package main

import (
	"sync"

	"github.com/google/uuid"

	. "vaijunto/shared"
)

/*
armazena trechos e id de uma viagem associada a um motorista,
ou de uma reserva associada a um passageiro
*/
type Viagem struct {
	ID      string
	Dono    string //pode ser motorista ou passageiro
	Trechos []*Trecho

	mu sync.Mutex
}

/*
trecho, usado para construir viagens e reservas bem como montar o grafo
armazena tambem os ids de reservas de passageiros que usam esse trecho,
permitindo facil rastreabilidade
*/
type Trecho struct {
	ViagemID  string //id da viagem de motorista ao qual esse trecho pertence
	ID        string
	Motorista string
	Origem    string
	Destino   string
	Vagas     int
	Preco     float64
	Data      string `json:"data"`
	Horario   string `json:"horario"`

	// viagens de passageiros que usam esse trecho
	ReservasAssociadasIDS []Par //ids das viagens associadas

	mu sync.Mutex
}

/*
converte trecho para trechodata, usado para enviar pacotes de informação ao cliente
*/
func (t *Trecho) ToTrechoData() TrechoData {
	t.mu.Lock()
	defer t.mu.Unlock()
	return TrechoData{
		ViagemID:             t.ViagemID,
		Motorista:            t.Motorista,
		ID:                   t.ID,
		Origem:               t.Origem,
		Destino:              t.Destino,
		Vagas:                t.Vagas,
		Preco:                t.Preco,
		Data:                 t.Data,
		Horario:              t.Horario,
		ViagensAssociadasIDS: t.ReservasAssociadasIDS,
	}
}

/*
transforma TrechoData em um Trecho
*/
func ToTrecho(t *TrechoData) Trecho {
	id := uuid.New().String()
	return Trecho{
		ID:        id,
		Motorista: t.Motorista,
		Origem:    t.Origem,
		Destino:   t.Destino,
		Vagas:     t.Vagas,
		Preco:     t.Preco,
		Data:      t.Data,
		Horario:   t.Horario,
	}
}

// transforma um slice de trecho data em um slice de Trecho para ser guardado no grafo
// quer um nome de funcao mais auto descritivo que esse?
/*
transforma um slice de trechodata em um objeto Viagem
usado após o cliente motorista construir a rota de trechos,
permitindo salvar no map e grafo
*/
func TrechoDataSliceToViagem(tds []TrechoData, dono string) Viagem {
	var viagem []*Trecho
	viagemid := uuid.New().String()

	for _, td := range tds {
		trecho := ToTrecho(&td)
		trecho.ViagemID = viagemid
		viagem = append(viagem, &trecho)
	}

	return Viagem{
		ID:      viagemid,
		Dono:    dono,
		Trechos: viagem,
	}
}

/*
usado para permitir enviar pacote de dados ao cliente
enquanto o passageiro decide qual reservar quer
*/
func (v *Viagem) ToViagemData() ViagemData {
	var vd []TrechoData

	for _, t := range v.Trechos {
		td := t.ToTrechoData()
		vd = append(vd, td)
	}

	return ViagemData{
		ID:      v.ID,
		Dono:    v.Dono,
		Trechos: vd,
	}
}

/*
usado para permitir enviar pacote de dados ao cliente
ao listar reservas e viagens para clientes
*/
func ViagemMapToViagemDataSlice(vs map[string]*Viagem) []ViagemData {
	var vds []ViagemData
	for _, v := range vs {
		vd := v.ToViagemData()
		vds = append(vds, vd)
	}
	return vds
}

/*
mapa de viagens associadas a usarios
*/
type MapaViagens struct {
	mu sync.Mutex
	//user-> ID da viagem-> Pointer da viagem
	Viagens map[string]map[string]*Viagem
}

func NovoViagens() *MapaViagens {
	return &MapaViagens{
		Viagens: make(map[string]map[string]*Viagem),
	}
}

func InicializarViagensByUser() map[string]*Viagem {
	return make(map[string]*Viagem)
}
