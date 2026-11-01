package testes

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	. "vaijunto/shared"
)

// ---------- 1. fluxo básico ----------

func TestReservaSimples(t *testing.T) {
	iniciarTeste(t, "reserva simples: 1 passageiro reserva 1 vaga, de ponta a ponta")

	ori, des := cidades()
	m := novoMotorista(t, srvDe(0), "mot"+uniq(), 2, ori, des)
	p := novoPassageiro(t, srvDe(1))

	r := p.must(t, Request{Acao: "BUSCAR_E_RESERVAR", Origem: ori, Destino: des})
	if r.Status != "SUCESSO" || len(r.Viagens) != 1 {
		t.Fatalf("busca: %+v", r)
	}
	if r = p.must(t, Request{Acao: "CONFIRMAR_ROTA"}); r.Status != "SUCESSO" {
		t.Fatalf("confirmação: %+v", r)
	}

	eventualmente(t, 3*time.Second, "vaga decrementada e reserva associada ao trecho", func() bool {
		tr := trechoDe(t, m)
		return tr.Vagas == 1 && len(tr.ViagensAssociadasIDS) == 1
	})
	nota(t, "a vaga do trecho caiu de 2 para 1 e a reserva ficou registrada nele")
}

// ---------- 2. prioridade por timestamp de Lamport ----------

/*
Envia prepares direto na API REST do servidor dono do trecho, com carimbos forjados:
B (ts=7, mais novo) chega primeiro; A (ts=5, mais antigo) chega 'atraso' depois.
Dentro da janela de ordenação, A tem que vencer.
*/
func TestPrioridadeLamport(t *testing.T) {
	iniciarTeste(t, "o pedido com carimbo Lamport mais antigo vence, mesmo chegando depois")

	srv := disponiveis[0]
	if !servidores[srv].httpOK {
		nota(t, "API REST do servidor %d inacessivel daqui (publique a porta HTTP)", srv)
		t.Skipf("API REST do servidor %d inacessível daqui (publique a porta HTTP)", srv)
	}
	ori, des := cidades()
	m := novoMotorista(t, srv, "mot"+uniq(), 1, ori, des) // 1 vaga: só um pode ganhar
	tr := trechoDe(t, m)

	rodada := func(atraso time.Duration) (aGanhou, bGanhou bool) {
		txA, txB := "txA-"+uniq()+uniq(), "txB-"+uniq()+uniq()
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			ok, err := prepareRest(srv, txB, tr, 7, 2)
			if err != nil {
				t.Errorf("prepare B: %v", err)
			}
			bGanhou = ok
		}()
		go func() {
			defer wg.Done()
			time.Sleep(atraso)
			ok, err := prepareRest(srv, txA, tr, 5, 3)
			if err != nil {
				t.Errorf("prepare A: %v", err)
			}
			aGanhou = ok
		}()
		wg.Wait()
		abortRest(srv, txA) // libera o lock de quem ganhou
		abortRest(srv, txB)
		return
	}

	// dentro da janela de ordenação: o mais antigo TEM que vencer, sempre
	total, venceuA := 0, 0
	for _, atraso := range []time.Duration{10 * time.Millisecond, 30 * time.Millisecond} {
		for i := 0; i < 10; i++ {
			a, b := rodada(atraso)
			total++
			if a && b {
				t.Fatalf("OVERBOOKING: dois prepares concedidos para 1 vaga")
			}
			if a && !b {
				venceuA++
			} else {
				t.Errorf("atraso %v, rodada %d: esperava vitória do ts mais antigo (A), mas A=%v B=%v", atraso, i, a, b)
			}
		}
	}
	nota(t, "chegando 10ms e 30ms depois (dentro da janela), o mais antigo venceu %d/%d (esperado: todos)", venceuA, total)
	nota(t, "nunca houve 2 concessoes para a mesma vaga")

	// fora da janela: sem garantia (é o limite do método), apenas informa
	vitA := 0
	for i := 0; i < 10; i++ {
		if a, _ := rodada(150 * time.Millisecond); a {
			vitA++
		}
	}
	nota(t, "chegando 150ms depois (fora da janela), o mais antigo venceu %d/10 (sem garantia; ~0 e o esperado)", vitA)
}

// ---------- 3. concorrência ponta a ponta ----------

type resultado struct {
	c     *cli
	saida string
}

func reservar(c *cli, ori, des string) string {
	r, err := c.call(Request{Acao: "BUSCAR_E_RESERVAR", Origem: ori, Destino: des})
	if err != nil {
		return "ERRO: " + err.Error()
	}
	if r.Status != "SUCESSO" {
		if strings.Contains(r.Message, "Nenhum itiner") {
			return "BUSCA_SEM_ROTA"
		}
		return "PREPARE_NEGADO"
	}
	r, err = c.call(Request{Acao: "CONFIRMAR_ROTA"})
	if err != nil {
		return "ERRO: " + err.Error()
	}
	if r.Status == "SUCESSO" {
		return "RESERVOU"
	}
	return "ERRO_CONFIRMA"
}

// n passageiros espalhados pelos servidores disponíveis disparam a busca+reserva no mesmo instante
func dispararReservas(t *testing.T, n int, ori, des string) []resultado {
	t.Helper()
	clientes := make([]*cli, n)
	for i := range clientes {
		clientes[i] = novoPassageiro(t, srvDe(i))
	}
	out := make([]resultado, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range clientes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			out[i] = resultado{clientes[i], reservar(clientes[i], ori, des)}
		}(i)
	}
	close(start)
	wg.Wait()
	return out
}

func contar(rs []resultado) map[string]int {
	c := map[string]int{}
	for _, r := range rs {
		c[r.saida]++
	}
	return c
}

// reservas confirmadas == vagas consumidas == reservas associadas, e nunca negativo
func checarConsistencia(t *testing.T, m *cli, vagasIniciais, reservou int) {
	t.Helper()
	if reservou > vagasIniciais {
		t.Fatalf("OVERBOOKING: %d reservas para %d vagas", reservou, vagasIniciais)
	}
	var tr TrechoData
	eventualmente(t, 3*time.Second, "vagas e reservas associadas consistentes", func() bool {
		tr = trechoDe(t, m)
		return len(tr.ViagensAssociadasIDS) == reservou && tr.Vagas == vagasIniciais-reservou
	})
	if tr.Vagas < 0 {
		t.Fatalf("vagas negativas: %d", tr.Vagas)
	}
}

// disputa por vagas escassas: pode negar gente, mas nunca pode vender além das vagas
func TestConcorrenciaSemOverbooking(t *testing.T) {
	const n, vagas = 12, 5
	iniciarTeste(t, fmt.Sprintf("%d passageiros disputam %d vagas ao mesmo tempo: nunca vender alem das vagas", n, vagas))

	ori, des := cidades()
	m := novoMotorista(t, srvDe(0), "mot"+uniq(), vagas, ori, des)

	c := contar(dispararReservas(t, n, ori, des))

	for k := range c {
		if strings.HasPrefix(k, "ERRO") {
			t.Errorf("resultado inesperado: %s (%d)", k, c[k])
		}
	}
	if c["RESERVOU"] == 0 {
		t.Errorf("ninguém conseguiu reservar")
	}
	checarConsistencia(t, m, vagas, c["RESERVOU"])
	nota(t, "%d reservaram e %d foram negados (de %d); vagas restantes: %d", c["RESERVOU"], n-c["RESERVOU"], n, vagas-c["RESERVOU"])
	nota(t, "reservas, vagas consumidas e associacoes do trecho bateram (sem overbooking)")
}

/*
Regressão do deadlock da busca: com vagas de sobra, a BUSCA nunca pode falhar.
(Antes da correção do RLock em BuscarRotas/GetAdjacencias, buscas simultâneas em
servidores diferentes se travavam e voltavam "nenhum itinerário".)
PREPARE_NEGADO ainda é esperado: o lock do trecho é exclusivo; só vira aviso.
*/
func TestBuscaConcorrenteSemFalsoNegativo(t *testing.T) {
	const n, vagas = 9, 12
	iniciarTeste(t, fmt.Sprintf("regressao do deadlock: %d buscas simultaneas nao podem falhar com %d vagas livres", n, vagas))
	exigir(t, 2) // o deadlock era um ciclo entre servidores; com 1 só não há como reproduzir

	ori, des := cidades()
	m := novoMotorista(t, srvDe(0), "mot"+uniq(), vagas, ori, des)

	c := contar(dispararReservas(t, n, ori, des))

	if c["BUSCA_SEM_ROTA"] > 0 {
		t.Errorf("%d buscas falharam com vagas sobrando (deadlock/timeout em /adjacencias?)", c["BUSCA_SEM_ROTA"])
	}
	checarConsistencia(t, m, vagas, c["RESERVOU"])
	nota(t, "%d buscas falharam (esperado: 0)", c["BUSCA_SEM_ROTA"])
	nota(t, "%d reservaram; vagas e reservas consistentes", c["RESERVOU"])
	if c["PREPARE_NEGADO"] > 0 {
		aviso(t, "o lock exclusivo do trecho negou %d pedidos mesmo com vagas sobrando (limitacao conhecida do design)", c["PREPARE_NEGADO"])
	}
}

// ---------- 4. cancelamentos com 2PC (reserva de 2 trechos em servidores diferentes) ----------

func TestCancelamentos2PC(t *testing.T) {
	iniciarTeste(t, "cancelar reserva e cancelar viagem (2PC) com trechos em 2 servidores")
	exigir(t, 2) // os dois trechos da reserva ficam em servidores diferentes

	sA, sB := disponiveis[0], disponiveis[1]
	sP := disponiveis[len(disponiveis)-1] // passageiro no 3º servidor, se existir
	x, y, z := "X"+uniq(), "Y"+uniq(), "Z"+uniq()
	mA := novoMotorista(t, sA, "mA"+uniq(), 2, x, y)
	mB := novoMotorista(t, sB, "mB"+uniq(), 2, y, z)
	p := novoPassageiro(t, sP)

	vagas := func(m *cli) int {
		vs := viagensDe(t, m)
		if len(vs) == 0 {
			return -1
		}
		return vs[0].Trechos[0].Vagas
	}
	reservarXZ := func() {
		r := p.must(t, Request{Acao: "BUSCAR_E_RESERVAR", Origem: x, Destino: z})
		if r.Status != "SUCESSO" || len(r.Viagens) != 1 || len(r.Viagens[0].Trechos) != 2 {
			t.Fatalf("busca X->Z deveria oferecer 1 itinerário de 2 trechos: %+v", r)
		}
		if r = p.must(t, Request{Acao: "CONFIRMAR_ROTA"}); r.Status != "SUCESSO" {
			t.Fatalf("confirmação: %+v", r)
		}
	}

	reservarXZ()
	eventualmente(t, 3*time.Second, "vagas 1/1 e 1 reserva", func() bool {
		return vagas(mA) == 1 && vagas(mB) == 1 && len(reservasDe(t, p)) == 1
	})

	// --- remoção de reserva pelo passageiro
	rid := reservasDe(t, p)[0].ID
	if r := p.must(t, Request{Acao: "REMOVER_RESERVA", ID: rid}); r.Status != "SUCESSO" {
		t.Fatalf("REMOVER_RESERVA: %+v", r)
	}
	eventualmente(t, 3*time.Second, "vagas devolvidas nos DOIS servidores", func() bool {
		return vagas(mA) == 2 && vagas(mB) == 2 && len(reservasDe(t, p)) == 0
	})
	nota(t, "passageiro remove a reserva: vagas devolvidas nos 2 servidores e reserva apagada")

	// --- cancelamento da viagem pelo motorista A, com a reserva ativa
	reservarXZ()
	eventualmente(t, 3*time.Second, "vagas 1/1 de novo", func() bool { return vagas(mA) == 1 && vagas(mB) == 1 })

	r := mA.must(t, Request{Acao: "REMOVER_VIAGEM", ID: viagensDe(t, mA)[0].ID})
	if r.Status != "SUCESSO" {
		t.Fatalf("REMOVER_VIAGEM: %+v", r)
	}
	eventualmente(t, 3*time.Second, "reserva apagada e vaga do motorista B devolvida", func() bool {
		return len(viagensDe(t, mA)) == 0 && vagas(mB) == 2 && len(reservasDe(t, p)) == 0
	})
	if n := p.must(t, Request{Acao: "GET_NOTI"}).Notificacoes; len(n) != 1 {
		t.Errorf("esperava 1 notificação, veio %d", len(n))
	}
	nota(t, "motorista cancela a viagem: reserva apagada, vaga do OUTRO motorista devolvida, 1 notificacao")

	// locks liberados: nova reserva no trecho de B funciona
	p2 := novoPassageiro(t, sA)
	if r := p2.must(t, Request{Acao: "BUSCAR_E_RESERVAR", Origem: y, Destino: z}); r.Status != "SUCESSO" {
		t.Errorf("trecho de B ficou travado após o cancelamento: %+v", r)
	}
	nota(t, "locks liberados: nova reserva no trecho do outro motorista funcionou")
}

// ---------- 5. motorista cancela a viagem enquanto passageiros reservam ----------

func TestCancelarViagemDuranteReservas(t *testing.T) {
	const n = 8
	iniciarTeste(t, fmt.Sprintf("motorista cancela a viagem enquanto %d passageiros reservam: sem reserva fantasma", n))

	for _, atraso := range []time.Duration{60, 100, 140, 180, 240} {
		atraso *= time.Millisecond
		t.Run(atraso.String(), func(t *testing.T) {
			ori, des := cidades()
			m := novoMotorista(t, srvDe(0), "mot"+uniq(), n, ori, des)
			vid := viagensDe(t, m)[0].ID

			clientes := make([]*cli, n)
			for i := range clientes {
				clientes[i] = novoPassageiro(t, srvDe(i))
			}
			saidas := make([]string, n)
			start := make(chan struct{})
			var wg sync.WaitGroup
			for i := range clientes {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-start
					saidas[i] = reservar(clientes[i], ori, des)
				}(i)
			}

			removida := false
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				time.Sleep(atraso)
				for tent := 0; tent < 10 && !removida; tent++ {
					r, err := m.call(Request{Acao: "REMOVER_VIAGEM", ID: vid})
					if err != nil {
						t.Errorf("REMOVER_VIAGEM: %v", err)
						return
					}
					removida = r.Status == "SUCESSO"
					if !removida {
						time.Sleep(150 * time.Millisecond)
					}
				}
			}()
			close(start)
			wg.Wait()

			if !removida {
				t.Fatalf("o motorista nunca conseguiu remover a viagem")
			}

			reservou := 0
			for i, c := range clientes {
				if strings.HasPrefix(saidas[i], "ERRO") {
					t.Errorf("passageiro %d: %s", i, saidas[i])
				}
				if saidas[i] == "RESERVOU" {
					reservou++
				}
				// invariante: nenhuma reserva fantasma e 1 notificação para quem tinha reservado
				want := 0
				if saidas[i] == "RESERVOU" {
					want = 1
				}
				eventualmente(t, 3*time.Second, "reservas removidas e notificação entregue", func() bool {
					return len(reservasDe(t, c)) == 0 &&
						len(c.must(t, Request{Acao: "GET_NOTI"}).Notificacoes) == want
				})
			}
			if len(viagensDe(t, m)) != 0 {
				t.Errorf("a viagem ainda existe")
			}
			if reservou == 0 {
				nota(t, "cancelou em +%v: antes de qualquer reserva confirmar (nada a cancelar)", atraso)
			} else {
				nota(t, "cancelou em +%v: %d reservas ja confirmadas, todas canceladas e notificadas", atraso, reservou)
			}
		})
	}
	nota(t, "em todos os instantes: nenhuma reserva fantasma e notificacoes corretas")
}
