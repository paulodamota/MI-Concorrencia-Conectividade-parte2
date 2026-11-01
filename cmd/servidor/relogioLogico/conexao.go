package relogioLogico

import (
	"net/http"
	"strconv"
)

const cabecalho = "X-Relogio"

type transporte struct {
	base http.RoundTripper
	l    *RelogioLogico
}

// Transporte: lado CLIENTE. Carimba toda requisição enviada e atualiza o relógio com a resposta.
func (l *RelogioLogico) Transporte(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return transporte{base: base, l: l}
}

func (t transporte) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context()) // RoundTripper não deve alterar a requisição original
	req.Header.Set(cabecalho, strconv.FormatInt(t.l.Evento(), 10))

	resp, err := t.base.RoundTrip(req)
	if err == nil {
		if v, e := strconv.ParseInt(resp.Header.Get(cabecalho), 10, 64); e == nil {
			t.l.Receber(v)
		}
	}
	return resp, err
}

// Middleware: lado SERVIDOR. Atualiza o relógio ao receber e devolve o valor no header da resposta.
func (l *RelogioLogico) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var t int64
		if v, err := strconv.ParseInt(r.Header.Get(cabecalho), 10, 64); err == nil {
			t = v
		}
		w.Header().Set(cabecalho, strconv.FormatInt(l.Receber(t), 10))
		next.ServeHTTP(w, r)
	})
}
