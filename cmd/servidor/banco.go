package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
)

type BancoData struct {
	Motoristas  map[string]*Usuario           `json:"motoristas"`
	Passageiros map[string]*Usuario           `json:"passageiros"`
	ViagensMoto map[string]map[string]*Viagem `json:"viagens_moto"`
	ViagensPass map[string]map[string]*Viagem `json:"viagens_pass"`
}

// caminho do arquivo
const arquivoBanco = "dados/dados_servidor.json"

/*
salva os dados atuais do servidor no arquivo
*/
func Salvar() {
	//pra guardar os dados tudo, temporario
	dados := BancoData{
		Motoristas:  make(map[string]*Usuario),
		Passageiros: make(map[string]*Usuario),

		//já copia viagens
		ViagensMoto: ViagensMoto.Viagens,
		ViagensPass: ViagensPass.Viagens,
	}

	// salva motoristas e passageiros
	Motoristas.Range(func(key, value interface{}) bool {
		dados.Motoristas[key.(string)] = value.(*Usuario)
		return true
	})

	Passageiros.Range(func(key, value interface{}) bool {
		dados.Passageiros[key.(string)] = value.(*Usuario)
		return true
	})

	// converte em json
	bytes, err := json.MarshalIndent(dados, "", "  ")
	if err != nil {
		log.Printf("Erro ao converter dados para JSON: %v", err)
		return
	}

	//escreve
	err = os.WriteFile(arquivoBanco, bytes, 0644)
	if err != nil {
		log.Printf("Erro ao salvar arquivo JSON: %v", err)
	}
}

/*
le o arquivo de dados e reconstroi o grafo
*/
func Carregar() {
	//le de volta do arquivo
	bytes, err := os.ReadFile(arquivoBanco)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("Arquivo de banco não encontrado. Iniciando servidor limpo.")
			return
		}
		log.Fatalf("Erro ao ler banco de dados: %v", err)
	}

	var dados BancoData
	if err := json.Unmarshal(bytes, &dados); err != nil {
		log.Fatalf("Erro ao decodificar JSON do banco: %v", err)
	}

	// restaura motoristas e passageiros e viagens
	for nome, user := range dados.Motoristas {
		user.LoggedIn = false
		Motoristas.Store(nome, user)
	}

	for nome, user := range dados.Passageiros {
		user.LoggedIn = false
		Passageiros.Store(nome, user)
	}

	// restaura viagens
	ViagensMoto.mu.Lock()
	ViagensMoto.Viagens = dados.ViagensMoto
	ViagensMoto.mu.Unlock()

	ViagensPass.mu.Lock()
	ViagensPass.Viagens = dados.ViagensPass
	ViagensPass.mu.Unlock()

	// renconstruindo grado
	for userMotorista, viagens := range ViagensMoto.Viagens {
		for _, viagem := range viagens {
			grafo.AdicionarViagem(userMotorista, viagem)
		}
	}

	log.Println("Dados restaurados com sucesso do arquivo JSON!")
}

/*
configuraa logs para salvar em servidor.log
*/
func ConfigurarLogs() *os.File {
	arquivo, err := os.OpenFile("dados/servidor.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Falha ao abrir o arquivo de log: %v", err)
	}

	//configura pra escrever tanto no terminal quanto no arquivo
	multiWriter := io.MultiWriter(os.Stdout, arquivo)

	log.SetOutput(multiWriter)

	//prefixos
	log.SetFlags(log.Ldate | log.Ltime)

	return arquivo
}
