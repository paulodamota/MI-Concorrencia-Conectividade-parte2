package main

import "sync"

type Usuario struct {
	Nome         string
	Senha        string
	LoggedIn     bool
	Notificacoes []string

	mu sync.Mutex
}

func NovoUsuario(nome string, senha string) *Usuario {
	return &Usuario{
		Nome:  nome,
		Senha: senha,
	}
}

func (u *Usuario) AdicionarNotificacao(mensagem string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.Notificacoes = append(u.Notificacoes, mensagem)
}
