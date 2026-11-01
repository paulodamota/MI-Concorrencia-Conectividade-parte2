// Package protocolo define o contrato HTTP entre servidores do consórcio.
package protocolo

import (
	. "vaijunto/cmd/servidor/relogioLogico"
	. "vaijunto/shared"
)

// simplesmente o pacote http
type PacoteHttp struct {
	Trechos     []TrechoData `json:"trechos,omitempty"`
	TransacaoID string       `json:"transacaoid,omitempty"`
	Reserva     ReservaChave `json:"passageiro,omitempty"`

	Reservas  []ReservaChave `json:"reservas,omitempty"`
	Motorista string         `json:"motorista,omitempty"`

	Voto bool `json:"voto,omitempty"`

	Timestamp Timestamp `json:"timestamp"`
}
