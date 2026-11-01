package main

import (
	"encoding/json"
	"fmt"
	"log"
	. "vaijunto/cmd/servidor/relogioLogico"
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

				ts := relogioDeLamport.Carimbo()
				err := loopEscolhareserva(itinerarios, user, encoder, decoder, &req, &res, ts)
				if err != nil {
					break //esse break é inutil kk
				}

			}

		case "REMOVER_RESERVA":
			{
				log.Printf("Solicitação de remoção de reserva de id: %s , pelo passageiro %s", req.ID, user)

				ok, msg := RemoverReserva2PC(user, req.ID)
				if !ok {
					res = Resposta{Status: "FALHA", Message: msg}
					break
				}
				res = Resposta{Status: "SUCESSO", Message: msg}
			}

		case "LISTAR_RESERVAS":
			{
				ReservasPass.mu.Lock()
				viagens := ReservasPass.Reservas[user]
				ReservasPass.mu.Unlock()

				mensagem := fmt.Sprintf("%d viagens cadastradas encontradas", len(viagens))

				res = Resposta{
					Status:  "SUCESSO",
					Message: mensagem,
					Viagens: ViagemDataMapToViagemDataSlice(viagens),
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
func loopEscolhareserva(itinerarios []*ViagemData, user string, encoder *json.Encoder, decoder *json.Decoder, req *Request, res *Resposta, ts Timestamp) error {
	var encerrar_loop bool
	for _, i := range itinerarios {
		var trecho_preparado ViagemData

		transacaoID, disponivel := ReservaPrepare(i, ts) //mudança necessaria para trechos entre servidores

		servidoresParticipantes := i.Servidores

		log.Printf("[PASS CON ESCOLHA RESERVA] servidores participantes: %v", servidoresParticipantes)

		if !disponivel {
			ReservaAbort(transacaoID, servidoresParticipantes)
			continue
		}

		log.Printf("Ola esperando usuario decidir sobre rota preparada")

		//++++++++++constroi e envia a resposta+++++++
		trecho_preparado = *i
		*res = Resposta{
			Status:  "SUCESSO",
			Message: "Busca de rota realizada com sucesso",
			Viagens: []ViagemData{trecho_preparado},
		}
		if err := encoder.Encode(res); err != nil {
			ReservaAbort(transacaoID, servidoresParticipantes)
			log.Printf("Erro ao enviar resposta: %v. Operação cancelada", err)
			return err
		}
		//++++++++++++++++++++++++++++++++++++++++++++

		//===========lê requisição do cliente============
		err := decoder.Decode(req)
		if err != nil {
			ReservaAbort(transacaoID, servidoresParticipantes)
			log.Printf("Operação cancelada. Conexão do passagerio '%s' encerrada, durante escolha da reserva: %v", user, err)
			return err
		}
		//===============================================

		switch req.Acao {
		case "CONFIRMAR_ROTA":
			{
				//o id da viagem eh adicionado aqui
				i.ID = transacaoID

				//necessita tratamento para queda de servidor aqui...
				ReservasPass.mu.Lock()
				ReservasPass.Reservas[user][i.ID] = i
				ReservasPass.mu.Unlock()

				ReservaCommit(transacaoID, user, servidoresParticipantes)

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
				ReservaAbort(transacaoID, servidoresParticipantes)
			}
		case "CANCELAR":
			{
				encerrar_loop = true
				ReservaAbort(transacaoID, servidoresParticipantes)
				*res = Resposta{
					Status:  "SUCESSO",
					Message: "Operação Cancelada",
				}
			}
		default:
			encerrar_loop = true
			ReservaAbort(transacaoID, servidoresParticipantes)
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
