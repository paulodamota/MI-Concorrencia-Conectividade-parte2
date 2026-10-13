package main

import (
	"sync"
	. "vaijunto/shared"
)

/*
Cadastro
verifica se já há usuario com aquele nome antes de permitir cadastro
ativa flag de login ao cadastra e fazer login do usario
*/
func Cadastrar(req Request, usuarios *sync.Map) Resposta {
	if req.Nome == "" || req.Senha == "" {
		return Resposta{
			Status:  "ERRO",
			Message: "Campo vazio",
		}
	}

	_, existe := usuarios.Load(req.Nome)
	if existe {
		return Resposta{
			Status:  "FALHA",
			Message: "Nome de usuario já cadastrado",
		}
	} else {
		usuarios.Store(req.Nome, NovoUsuario(req.Nome, req.Senha))

		usuario, _ := usuarios.Load(req.Nome)
		user := usuario.(*Usuario)
		user.LoggedIn = true

		return Resposta{
			Status:  "SUCESSO",
			Message: "Cadastro concluido",
		}
	}

}

/*
login
verifica se o usario existe e se senha informada esta corrreta
*/
func Login(req Request, usuarios *sync.Map) Resposta {
	if req.Nome == "" || req.Senha == "" {
		return Resposta{
			Status:  "ERRO",
			Message: "Campo vazio",
		}
	}

	usuario, existe := usuarios.Load(req.Nome)
	// resposta de usuario não exitente
	if !existe {
		return Resposta{
			Status:  "FALHA",
			Message: "Usuario não encontrado",
		}

		// resposta de senha errada
	} else if usuario.(*Usuario).Senha != req.Senha {
		return Resposta{
			Status:  "FALHA",
			Message: "Senha incorreta",
		}

		// resposta de login concluido
	} else {
		user := usuario.(*Usuario)

		user.mu.Lock()
		defer user.mu.Unlock()

		if user.LoggedIn {
			return Resposta{
				Status:  "FALHA",
				Message: "Usuario já logado",
			}
		}

		user.LoggedIn = true

		return Resposta{
			Status:  "SUCESSO",
			Message: "Login concluido",
		}
	}
}
