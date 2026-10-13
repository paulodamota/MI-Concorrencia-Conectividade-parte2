end ?= host.docker.internal:6742

.PHONY: serv moto pass

serv:
	@echo "Inicializando container servidor..."
	docker build -t server-img -f Dockerfile.servidor .
	-docker stop server-cont 2>/dev/null || true
	-docker rm server-cont 2>/dev/null || true
	docker run -it -p 6742:6742 -v "$(PWD)/dados:/app/dados" --name server-cont server-img

moto:
	@echo "Inicializando container do motorista..."
	docker build -t moto-img -f Dockerfile.motorista .
	-docker stop moto-cont 2>/dev/null || true
	-docker rm moto-cont 2>/dev/null || true
	docker run -it -e endereco="$(end):6742" --name moto-cont moto-img

pass:
	@echo "Inicializando container do passageiro..."
	docker build -t pass-img -f Dockerfile.passageiro .
	-docker stop pass-cont 2>/dev/null || true
	-docker rm pass-cont 2>/dev/null || true
	docker run -it -e endereco="$(end):6742" --name pass-cont pass-img
