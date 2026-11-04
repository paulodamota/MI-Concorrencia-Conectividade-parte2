package testes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	. "vaijunto/shared"
)

/*
Os testes NÃO sobem servidores. Suba os containers e informe onde cada servidor está:

	SERVIDOR1=<host>[:<porta tcp>[:<porta http>]]
	SERVIDOR2=...
	SERVIDOR3=...

Portas padrão (as mesmas do servidor): 1 -> 2800/2828, 2 -> 2900/2929, 3 -> 3100/3131.
Variável ausente, ou servidor inacessível, significa "servidor fora do ar": os testes usam
apenas os servidores disponíveis e pulam (SKIP) o que precisa de mais.
Observação: são endereços acessíveis a partir de ONDE OS TESTES RODAM (host, IP do
container, ou nome do serviço se os testes rodarem dentro da rede do Docker).
*/

type servidorInfo struct {
	host   string
	tcp    string
	http   string
	httpOK bool // porta HTTP (REST entre servidores) alcançável daqui
}

var (
	servidores  = map[int]*servidorInfo{}
	disponiveis []int // IDs (1..3) alcançáveis por TCP, em ordem crescente
)

var portasPadrao = map[int][2]string{1: {"2800", "2828"}, 2: {"2900", "2929"}, 3: {"3100", "3131"}}

func lerServidor(id int) *servidorInfo {
	v := strings.TrimSpace(os.Getenv(fmt.Sprintf("SERVIDOR%d", id)))
	if v == "" {
		return nil
	}
	p := strings.Split(v, ":")
	s := &servidorInfo{host: p[0], tcp: portasPadrao[id][0], http: portasPadrao[id][1]}
	if len(p) > 1 && p[1] != "" {
		s.tcp = p[1]
	}
	if len(p) > 2 && p[2] != "" {
		s.http = p[2]
	}
	return s
}

func TestMain(m *testing.M) {
	for id := 1; id <= 3; id++ {
		s := lerServidor(id)
		if s == nil {
			fmt.Printf("servidor %d: SERVIDOR%d não definida -> considerado fora do ar\n", id, id)
			rel.cabecalho = append(rel.cabecalho, fmt.Sprintf("Servidor %d: fora do ar (SERVIDOR%d nao definida)", id, id))
			continue
		}
		if !esperarPorta(s.host+":"+s.tcp, 3*time.Second) {
			fmt.Printf("servidor %d: %s:%s inacessível (TCP) -> considerado fora do ar\n", id, s.host, s.tcp)
			rel.cabecalho = append(rel.cabecalho, fmt.Sprintf("Servidor %d: fora do ar (%s:%s nao responde)", id, s.host, s.tcp))
			continue
		}
		s.httpOK = esperarPorta(s.host+":"+s.http, time.Second)
		if !s.httpOK {
			fmt.Printf("aviso: servidor %d: HTTP %s:%s inacessível daqui (testes que usam a REST serão ignorados)\n", id, s.host, s.http)
		}
		servidores[id] = s
		disponiveis = append(disponiveis, id)
	}
	if len(disponiveis) == 0 {
		fmt.Println("nenhum servidor acessível. Exemplo: SERVIDOR1=localhost SERVIDOR2=localhost SERVIDOR3=localhost go test ./testes/ -v")
		os.Exit(1)
	}
	fmt.Printf("servidores em teste: %v\n", disponiveis)
	rel.cabecalho = append(rel.cabecalho, fmt.Sprintf("Servidores em teste: %v", disponiveis))
	code := m.Run()
	imprimirRelatorio()
	os.Exit(code)
}

func esperarPorta(addr string, limite time.Duration) bool {
	fim := time.Now().Add(limite)
	for time.Now().Before(fim) {
		if c, err := net.DialTimeout("tcp", addr, 500*time.Millisecond); err == nil {
			c.Close()
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func tcpAddr(srv int) string  { s := servidores[srv]; return s.host + ":" + s.tcp }
func httpBase(srv int) string { s := servidores[srv]; return "http://" + s.host + ":" + s.http }

// i-ésimo servidor disponível, circular (espalha clientes entre os que existem)
func srvDe(i int) int { return disponiveis[i%len(disponiveis)] }

// pula o teste se houver menos de n servidores
func exigir(t testing.TB, n int) {
	t.Helper()
	if len(disponiveis) < n {
		nota(t, "precisa de %d servidores no ar e so ha %d %v", n, len(disponiveis), disponiveis)
		t.Skipf("precisa de %d servidores no ar, há %d (%v)", n, len(disponiveis), disponiveis)
	}
}

// ---------- cliente TCP (mesmo protocolo dos clientes reais) ----------

type cli struct {
	conn net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
}

func novoCli(t testing.TB, srv int) *cli {
	t.Helper()
	conn, err := net.DialTimeout("tcp", tcpAddr(srv), 3*time.Second)
	if err != nil {
		t.Fatalf("conectar no servidor %d (%s): %v", srv, tcpAddr(srv), err)
	}
	t.Cleanup(func() { conn.Close() })
	return &cli{conn: conn, enc: json.NewEncoder(conn), dec: json.NewDecoder(conn)}
}

// seguro para usar em goroutines (não chama t.Fatal)
func (c *cli) call(req Request) (Resposta, error) {
	var res Resposta
	c.conn.SetDeadline(time.Now().Add(15 * time.Second))
	if err := c.enc.Encode(req); err != nil {
		return res, err
	}
	err := c.dec.Decode(&res)
	return res, err
}

func (c *cli) must(t testing.TB, req Request) Resposta {
	t.Helper()
	res, err := c.call(req)
	if err != nil {
		t.Fatalf("%s: %v", req.Acao, err)
	}
	return res
}

// ---------- helpers de cenário ----------

// 32 bits: os servidores são persistentes (containers), então nomes/rotas não podem colidir entre execuções
func uniq() string { return fmt.Sprintf("%08x", rand.Uint32()) }

// rota única por teste, para que um teste não reserve em viagens de outro
func cidades() (string, string) { return "O" + uniq(), "D" + uniq() }

func novoMotorista(t testing.TB, srv int, nome string, vagas int, origem, destino string) *cli {
	t.Helper()
	m := novoCli(t, srv)
	if r := m.must(t, Request{Acao: "CADASTRO_MOTORISTA", Nome: nome, Senha: "x"}); r.Status != "SUCESSO" {
		t.Fatalf("cadastro de motorista: %+v", r)
	}
	r := m.must(t, Request{Acao: "CADASTRAR_ROTA", Origem: origem, Destino: destino, Trechos: []TrechoData{{
		Motorista: nome, Origem: origem, Destino: destino, Vagas: vagas, Preco: 10, Data: "01-12-2026", Horario: "08:00",
	}}})
	if r.Status != "SUCESSO" {
		t.Fatalf("cadastrar rota: %+v", r)
	}
	return m
}

func novoPassageiro(t testing.TB, srv int) *cli {
	t.Helper()
	p := novoCli(t, srv)
	if r := p.must(t, Request{Acao: "CADASTRO_PASSAGEIRO", Nome: "p" + uniq(), Senha: "x"}); r.Status != "SUCESSO" {
		t.Fatalf("cadastro de passageiro: %+v", r)
	}
	return p
}

func viagensDe(t testing.TB, m *cli) []ViagemData {
	t.Helper()
	return m.must(t, Request{Acao: "LISTAR_ROTAS"}).Viagens
}

// primeiro trecho da primeira viagem do motorista
func trechoDe(t testing.TB, m *cli) TrechoData {
	t.Helper()
	vs := viagensDe(t, m)
	if len(vs) == 0 || len(vs[0].Trechos) == 0 {
		t.Fatalf("motorista sem viagens")
	}
	return vs[0].Trechos[0]
}

func reservasDe(t testing.TB, p *cli) []ViagemData {
	t.Helper()
	return p.must(t, Request{Acao: "LISTAR_RESERVAS"}).Viagens
}

func eventualmente(t testing.TB, limite time.Duration, desc string, cond func() bool) {
	t.Helper()
	fim := time.Now().Add(limite)
	for time.Now().Before(fim) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("não ocorreu em %v: %s", limite, desc)
}

// ---------- API REST entre servidores ----------

var httpCli = &http.Client{Timeout: 3 * time.Second}

func postRest(srv int, caminho string, corpo any) (status int, voto bool, err error) {
	b, _ := json.Marshal(corpo)
	resp, err := httpCli.Post(httpBase(srv)+caminho, "application/json", bytes.NewReader(b))
	if err != nil {
		return 0, false, err
	}
	defer resp.Body.Close()
	var r struct {
		Voto bool `json:"voto"`
	}
	json.NewDecoder(resp.Body).Decode(&r)
	return resp.StatusCode, r.Voto, nil
}

func prepareRest(srv int, tx string, tr TrechoData, tempo int64, origemTs int) (bool, error) {
	st, voto, err := postRest(srv, "/reserva/prepare", map[string]any{
		"transacaoid": tx,
		"trechos":     []TrechoData{tr},
		"timestamp":   map[string]any{"tempo": tempo, "servidor": origemTs},
	})
	return err == nil && st == http.StatusOK && voto, err
}

func abortRest(srv int, tx string) {
	postRest(srv, "/reserva/abort", map[string]any{"transacaoid": tx})
}

// ---------- relatório final ----------
// (texto do relatório sem acentos de propósito: evita caracteres quebrados no console do Windows)

type linhaRel struct {
	nome, objetivo, resultado string
	detalhes                  []string
	dur                       time.Duration
}

var rel = struct {
	mu        sync.Mutex
	linhas    []*linhaRel
	porNome   map[string]*linhaRel
	avisos    []string
	cabecalho []string
}{porNome: map[string]*linhaRel{}}

// registra o teste no relatório: chamar na PRIMEIRA linha de cada teste
func iniciarTeste(t *testing.T, objetivo string) {
	t.Helper()
	l := &linhaRel{nome: t.Name(), objetivo: objetivo}
	rel.mu.Lock()
	rel.linhas = append(rel.linhas, l)
	rel.porNome[l.nome] = l
	rel.mu.Unlock()

	inicio := time.Now()
	t.Cleanup(func() {
		rel.mu.Lock()
		defer rel.mu.Unlock()
		l.dur = time.Since(inicio)
		switch {
		case t.Failed():
			l.resultado = "FALHOU"
		case t.Skipped():
			l.resultado = "PULADO"
		default:
			l.resultado = "OK"
		}
	})
}

// acrescenta uma linha de resultado ao teste (subtestes escrevem na linha do teste pai)
func nota(t testing.TB, formato string, args ...any) {
	raiz := strings.SplitN(t.Name(), "/", 2)[0]
	rel.mu.Lock()
	defer rel.mu.Unlock()
	if l := rel.porNome[raiz]; l != nil {
		l.detalhes = append(l.detalhes, fmt.Sprintf(formato, args...))
	}
}

// ponto de atenção que aparece no fim do relatório (não faz o teste falhar)
func aviso(t testing.TB, formato string, args ...any) {
	rel.mu.Lock()
	defer rel.mu.Unlock()
	rel.avisos = append(rel.avisos, fmt.Sprintf("%s: %s", strings.SplitN(t.Name(), "/", 2)[0], fmt.Sprintf(formato, args...)))
}

func imprimirRelatorio() {
	rel.mu.Lock()
	defer rel.mu.Unlock()

	barra := strings.Repeat("=", 78)
	fmt.Println()
	fmt.Println(barra)
	fmt.Println("RELATORIO DOS TESTES")
	fmt.Println(barra)
	for _, c := range rel.cabecalho {
		fmt.Println(c)
	}

	ok, falhou, pulado := 0, 0, 0
	for _, l := range rel.linhas {
		res := l.resultado
		if res == "" { // teste interrompido (panic/timeout) antes do Cleanup
			res = "FALHOU"
		}
		marca := map[string]string{"OK": "[  OK  ]", "FALHOU": "[FALHOU]", "PULADO": "[PULADO]"}[res]
		switch res {
		case "OK":
			ok++
		case "FALHOU":
			falhou++
		default:
			pulado++
		}

		fmt.Printf("\n%s %s  (%.2fs)\n", marca, l.nome, l.dur.Seconds())
		fmt.Printf("   Objetivo : %s\n", l.objetivo)
		if res == "FALHOU" {
			fmt.Println("   Resultado: FALHOU - veja a mensagem de erro (e2e_test.go:linha) mais acima")
			continue
		}
		rotulo := "Resultado:"
		if res == "PULADO" {
			rotulo = "Motivo   :"
		}
		for i, d := range l.detalhes {
			if i == 0 {
				fmt.Printf("   %s %s\n", rotulo, d)
			} else {
				fmt.Printf("              %s\n", d)
			}
		}
	}

	if len(rel.avisos) > 0 {
		fmt.Println()
		fmt.Println("PONTOS DE ATENCAO (nao sao falhas):")
		for _, a := range rel.avisos {
			fmt.Println("  - " + a)
		}
	}

	fmt.Println()
	fmt.Println(barra)
	fmt.Printf("TOTAL: %d testes | %d OK | %d FALHOU | %d PULADO\n", len(rel.linhas), ok, falhou, pulado)
	fmt.Println(barra)
}
