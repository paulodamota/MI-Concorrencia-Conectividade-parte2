package main

import (
	"encoding/json"
	"fmt"
	"log"
	. "vaijunto/shared"
)

func PassageiroConn(encoder *json.Encoder, decoder *json.Decoder, user string) {
	for {
		var logout bool
		var req Request
		err := decoder.Decode(&req)

		if err != nil {
			pass, _ := Passageiros.Load(user)
			pass.(*Usuario).LoggedIn = false
			log.Printf("Conexão de passageiro '%s' encerrada no handle com erro: %v", user, err)
			break
		}

		var res Resposta

		switch req.Acao {
		case "BUSCAR_E_RESERVAR":
			{
				log.Printf("Tentativa de reserva de rota enviada pelo passageiro %s: Origem: %s, Destino: %s", user, req.Origem, req.Destino)

				itinerarios := grafo.BuscarRotas(req.Origem, req.Destino, req.Data)

				//caso não haja rotas
				if len(itinerarios) == 0 {
					res = Resposta{
						Status:  "FALHA",
						Message: "Nenhum itinerário encontrado para esta rota.",
					}
					break
				}

				err := loopEscolhareserva(itinerarios, user, encoder, decoder, &req, &res)
				if err != nil {
					break //esse break é inutil kk
				}

			}

		case "REMOVER_RESERVA":
			{
				log.Printf("Solicitação de remoção de reserva de id: %s , pelo passageiro %s", req.ID, user)

				ViagensPass.mu.Lock()
				//talvez devesse ter um if aqui ?
				//para garantir que existe map e viagem
				viagens := ViagensPass.Viagens[user]
				paraRemover, existe := viagens[req.ID]
				if existe {
					delete(viagens, req.ID)
				}
				ViagensPass.mu.Unlock()

				if !existe || paraRemover == nil {
					res = Resposta{
						Status:  "ERRO",
						Message: "Reserva não encontrada.",
					}
					break
				}

				grafo.LiberarVagas(paraRemover)

				res = Resposta{
					Status:  "SUCESSO",
					Message: "Reserva removida com sucesso",
				}

			}

		case "LISTAR_RESERVAS":
			{
				ViagensPass.mu.Lock()
				viagens := ViagensPass.Viagens[user]
				ViagensPass.mu.Unlock()

				mensagem := fmt.Sprintf("%d viagens cadastradas encontradas", len(viagens))

				res = Resposta{
					Status:  "SUCESSO",
					Message: mensagem,
					Viagens: ViagemMapToViagemDataSlice(viagens),
				}
			}

		case "GET_NOTI":
			{
				passageiro, _ := Passageiros.Load(user)
				noti := passageiro.(*Usuario).Notificacoes
				mensagem := fmt.Sprintf("Notificações encontradas: [%d]", len(noti))
				res = Resposta{
					Status:       "SUCESSO",
					Message:      mensagem,
					Notificacoes: noti,
				}

			}

		case "LOG_OUT":
			{
				res = Resposta{
					Status:  "SUCESSO",
					Message: "Usuário será desconectado da conta",
				}

				logout = true
			}

		default:
			log.Printf("Ação de passageiro inválida ou não reconhecida: %s", req.Acao)
			res = Resposta{
				Status:  "ERRO",
				Message: "Ação inválida para passageiro",
			}
		}

		if err := encoder.Encode(res); err != nil {
			log.Printf("Erro ao enviar resposta para passageiro: %v", err)
			break
		}

		if logout {

			usuario, _ := Passageiros.Load(user)

			usuario.(*Usuario).mu.Lock()
			defer usuario.(*Usuario).mu.Unlock()

			usuario.(*Usuario).LoggedIn = false

			log.Printf("Passageiro '%s' fez logout ", user)

			return
		}
	}
}

/*
loop para tratar a escoha da reserva do passageiro
*/
func loopEscolhareserva(itinerarios []*Viagem, user string, encoder *json.Encoder, decoder *json.Decoder, req *Request, res *Resposta) error {
	var encerrar_loop bool
	for _, i := range itinerarios {
		var trecho_pre_reservado []ViagemData

		reservado := grafo.TentarReservarItinerario(i, user)

		if !reservado {
			continue
		}

		//++++++++++constroi e envia a resposta+++++++
		trecho_pre_reservado = append(trecho_pre_reservado, i.ToViagemData())
		*res = Resposta{
			Status:  "SUCESSO",
			Message: "Busca de rota realizada com sucesso",
			Viagens: trecho_pre_reservado,
		}
		if err := encoder.Encode(res); err != nil {
			grafo.LiberarVagas(i)
			log.Printf("Erro ao enviar resposta: %v. Operação encerrado", err)
			return err
		}
		//++++++++++++++++++++++++++++++++++++++++++++

		//===========lê requisição do cliente============
		err := decoder.Decode(req)
		if err != nil {
			grafo.LiberarVagas(i) //eu já libero quando erro, porqueeu sou tão triste
			log.Printf("Conexão do passagerio '%s' encerrada, durante escolha da reserva: %v", user, err)
			return err
		}
		//===============================================

		switch req.Acao {
		case "CONFIRMAR_ROTA":
			{

				ViagensPass.mu.Lock()
				ViagensPass.Viagens[user][i.ID] = i
				ViagensPass.mu.Unlock()

				encerrar_loop = true

				log.Printf("Reserva de id %s realizada com sucesso para o passageiro '%s'", i.ID, user)
				*res = Resposta{
					Status:  "SUCESSO",
					Message: "Itinerário reservado com sucesso!",
				}

			}
		case "PROXIMA_ROTA":
			{
				//A resposta ao ciente será dada na proxima iteração do loop
				// ja com a proxima rota indexada
				grafo.LiberarVagas(i)
			}
		case "CANCELAR":
			{
				encerrar_loop = true
				grafo.LiberarVagas(i)
				*res = Resposta{
					Status:  "SUCESSO",
					Message: "Operação Cancelada",
				}
			}
		default:
			encerrar_loop = true
			grafo.LiberarVagas(i)
			*res = Resposta{
				Status:  "ERRO",
				Message: fmt.Sprintf("Acao não reconhecida: '%s' . Operação Cancelada", req.Acao),
			}
		}

		//se o cliente escolheu a rota atual (ou cancelou a operação), quebra o loop
		if encerrar_loop {
			return nil
		}
	}
	//o loop pode ter terminado pq o usuario cancelou ou escolheu, e houve break, ou por ter esgotados as opções
	if req.Acao == "PROXIMA_ROTA" {
		*res = Resposta{
			Status:  "FALHA",
			Message: "Não há mais opcoes de rota, operação encerrada.",
		}
	}

	return nil

}
