NUM ?= 1
IP ?= 172.16.103.2

PASTA_DADOS = $(PWD)/dados$(NUM)
NOME_CONTAINER = server$(NUM)

ifeq ($(NUM),1)
    PORTAS_MAP = -p 2800:2800 -p 2828:2828
else ifeq ($(NUM),2)
    PORTAS_MAP = -p 2900:2900 -p 2929:2929
else ifeq ($(NUM),3)
    PORTAS_MAP = -p 3100:3100 -p 3131:3131
else
    PORTAS_MAP = -p 8080:8080
endif

.PHONY: serv
serv:
	@mkdir -p $(PASTA_DADOS)
	docker build -t server-img -f Dockerfile.servidor .
	-docker stop $(NOME_CONTAINER) 2>/dev/null || true
	-docker rm $(NOME_CONTAINER) 2>/dev/null || true
	docker run -it $(PORTAS_MAP) \
		-e servidor="$(NUM)" \
		-e endereco1="$(IP)" \
		-e endereco2="$(IP)" \
		-e endereco3="$(IP)" \
		-v "$(PASTA_DADOS):/app/dados" \
		--name $(NOME_CONTAINER) server-img

.PHONY: moto
moto:
	@echo "Incializando container do motorista..."
	docker build -t moto-img -f Dockerfile.motorista .
	-docker stop moto$(NUM) 2>/dev/null || true
	-docker rm moto$(NUM) 2>/dev/null || true
	docker run -it -e endereco="$(IP):$(PORT)" --name moto$(NUM) moto-img

.PHONY: pass
pass:
	@echo "Inicializando container do passageiro..."
	docker build -t pass-img -f Dockerfile.passageiro .
	-docker stop pass$(NUM) 2>/dev/null || true
	-docker rm pass$(NUM) 2>/dev/null || true
	docker run -it -e endereco="$(IP):$(PORT)" --name pass$(NUM) pass-img