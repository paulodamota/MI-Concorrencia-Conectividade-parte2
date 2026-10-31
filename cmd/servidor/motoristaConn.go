package main

import (
	"encoding/json"
	"fmt"
	"log"

	. "vaijunto/shared"
)

/*
tratamento de conexão de motorista
*/
func MotoristaConn(encoder *json.Encoder, decoder *json.Decoder, user string) {
	for {
		var logout bool
		var req Request
		err := decoder.Decode(&req)

		//dar uma olhada nisso depois, pra tratar encerramento da conexão direito
		if err != nil {
			moto, _ := Motoristas.Load(user)
			moto.(*Usuario).LoggedIn = false
			log.Printf("Conexão de motorista '%s' encerrada no handle com erro: %v", user, err)
			break
		}

		var res Resposta

		switch req.Acao {
		case "CADASTRAR_ROTA":
			{
				//todos os trechos são criados aqui
				log.Printf("Lista de trechos enviada pelo motorista '%s' \nParadas %d", user, len(req.Trechos))

				viagem := TrechoDataSliceToViagem(req.Trechos, user, true)

				grafo.AdicionarViagem(user, &viagem)

				ViagensMoto.mu.Lock()
				ViagensMoto.Viagens[user][viagem.ID] = &viagem
				ViagensMoto.mu.Unlock()

				res = Resposta{
					Status:  "SUCESSO",
					Message: "Rota cadastrada",
				}
			}

		case "REMOVER_VIAGEM":
			{
				log.Printf("Solicitação de remoção de viagem de id: '%s' , pelo motorista '%s'", req.ID, user)

				ok, msg := RemoverViagem2PC(user, req.ID)
				if !ok {
					res = Resposta{Status: "FALHA", Message: msg}
					break
				}
				res = Resposta{Status: "SUCESSO", Message: msg}

			}

		case "LISTAR_ROTAS":
			{
				ViagensMoto.mu.Lock()
				viagens := ViagensMoto.Viagens[user]
				ViagensMoto.mu.Unlock()

				mensagem := fmt.Sprintf("%d viagens cadastradas encontradas", len(viagens))

				res = Resposta{
					Status:  "SUCESSO",
					Message: mensagem,
					Viagens: ViagemMapToViagemDataSlice(viagens),
				}
			}

		case "LOG_OUT":
			res = Resposta{
				Status:  "SUCESSO",
				Message: "Usuario será desconectado da conta",
			}
			logout = true

		default:
			res = Resposta{
				Status:  "ERRO",
				Message: fmt.Sprintf("Acao não reconhecida: '%s' . Operação Cancelada", req.Acao),
			}

		}

		if err := encoder.Encode(res); err != nil {
			log.Printf("Erro ao enviar resposta: %v", err)
			break
		}

		if logout {
			usuario, _ := Motoristas.Load(user)

			usuario.(*Usuario).mu.Lock()
			defer usuario.(*Usuario).mu.Unlock()

			usuario.(*Usuario).LoggedIn = false

			log.Printf("Motorista '%s' fez logout ", user)

			return
		}

	}
}
