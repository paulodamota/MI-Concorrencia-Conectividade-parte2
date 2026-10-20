package main

import (
	"encoding/json"
	"fmt"
	"log"
	"slices"

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
				log.Printf("Solicitação de remoção de reserva de id: '%s' , pelo motorista '%s'", req.ID, user)

				ViagensMoto.mu.Lock()
				viagens := ViagensMoto.Viagens[user]
				paraRemover, existe := viagens[req.ID]
				if existe {
					delete(viagens, req.ID)
				}
				ViagensMoto.mu.Unlock()

				if !existe || paraRemover == nil {
					res = Resposta{
						Status:  "ERRO",
						Message: "Reserva não encontrada.",
					}
					break
				}

				viagensPassageirosAfetadas := grafo.RemoverViagemMotorista(paraRemover)

				if len(viagensPassageirosAfetadas) > 0 {

					ViagensPass.mu.Lock()
					for _, viagemAfetada := range viagensPassageirosAfetadas {
						passageiroAfetado := viagemAfetada.User

						passAfet, _ := Passageiros.Load(passageiroAfetado)

						msg := fmt.Sprintf("Sua reserva de ID '%s' foi cancelada pois um de seus motorista cancelou a viagem!\n Em breve você receberá reemboloso de sua reserva. Agradecemos a compreensão!\n", viagemAfetada.ID)
						if !slices.Contains(passAfet.(*Usuario).Notificacoes, msg) {
							passAfet.(*Usuario).AdicionarNotificacao(msg)
						}

						reservas := ViagensPass.Viagens[passageiroAfetado]
						grafo.LiberarVagas(reservas[viagemAfetada.ID]) //devolvendo vagas para cada trecho da vaga antes de remover
						delete(reservas, viagemAfetada.ID)
					}
				}
				ViagensPass.mu.Unlock()

				log.Printf("%d reservas de passageiros foram canceladas devido à exclusão da viagem %s", len(viagensPassageirosAfetadas), req.ID)

				res = Resposta{
					Status:  "SUCESSO",
					Message: "Viagem removida com sucesso. Quaisquer reservas dependentes foram canceladas.",
				}

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
