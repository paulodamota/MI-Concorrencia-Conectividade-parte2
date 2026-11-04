NUM ?= 1
IP1 ?= 192.168.15.111
IP2 ?= 192.168.15.112
IP3 ?= 192.168.15.113

PASTA_DADOS = $(PWD)/dados$(NUM)

ifeq ($(NUM),1)
    PORTAS_MAP = -p 2800:2800 -p 2828:2828
    IP_ATUAL = $(IP1)
    PORTA_ATUAL = 2800
else ifeq ($(NUM),2)
    PORTAS_MAP = -p 2900:2900 -p 2929:2929
    IP_ATUAL = $(IP2)
    PORTA_ATUAL = 2900
else ifeq ($(NUM),3)
    PORTAS_MAP = -p 3100:3100 -p 3131:3131
    IP_ATUAL = $(IP3)
    PORTA_ATUAL = 3100
else
    PORTAS_MAP = -p 8080:8080
    IP_ATUAL = $(IP1)
    PORTA_ATUAL = 8080
endif

.PHONY: serv
serv:
	@mkdir -p $(PASTA_DADOS)
	docker build -t server-img -f Dockerfile.servidor .
	-docker stop server$(NUM) 2>/dev/null || true
	-docker rm server$(NUM) 2>/dev/null || true
	docker run -it $(PORTAS_MAP) \
		-e servidor="$(NUM)" \
		-e endereco1="$(IP1)" \
		-e endereco2="$(IP2)" \
		-e endereco3="$(IP3)" \
		-v "$(PASTA_DADOS):/app/dados" \
		--name server$(NUM) server-img

.PHONY: moto
moto:
	@echo "Incializando container do motorista..."
	docker build -t moto-img -f Dockerfile.motorista .
	-docker stop moto$(NUM) 2>/dev/null || true
	-docker rm moto$(NUM) 2>/dev/null || true
	docker run -it -e endereco="$(IP_ATUAL):$(PORTA_ATUAL)" --name moto$(NUM) moto-img

.PHONY: pass
pass:
	@echo "Inicializando container do passageiro..."
	docker build -t pass-img -f Dockerfile.passageiro .
	-docker stop pass$(NUM) 2>/dev/null || true
	-docker rm pass$(NUM) 2>/dev/null || true
	docker run -it -e endereco="$(IP_ATUAL):$(PORTA_ATUAL)" --name pass$(NUM) pass-img