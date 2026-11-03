:: attserv.bat
set NUM_SERVIDOR=%1

set PASTA_DADOS=%cd%\dados%NUM_SERVIDOR%
if not exist "%PASTA_DADOS%" mkdir "%PASTA_DADOS%"

:: 3. Define as portas mapeadas (-p) com base no número do servidor
if "%NUM_SERVIDOR%"=="1" (
    set PORTAS_MAP=-p 2800:2800 -p 2828:2828
    set NOME_CONTAINER=server1
) else if "%NUM_SERVIDOR%"=="2" (
   set PORTAS_MAP=-p 2900:2900 -p 2929:2929
   set NOME_CONTAINER=server2
) else if "%NUM_SERVIDOR%"=="3" (
    set PORTAS_MAP=-p 3100:3100 -p 3131:3131
    set NOME_CONTAINER=server3
) else (
    :: Porta padrão caso seja outro número
    set PORTAS_MAP=-p 8080:8080
)

docker build -t server-img -f Dockerfile.servidor .

docker stop %NOME_CONTAINER%
docker rm %NOME_CONTAINER%

docker run -it %PORTAS_MAP% -e servidor="%NUM_SERVIDOR%" -e endereco1="192.168.15.111" -e endereco2="192.168.15.111" -e endereco3="192.168.15.111" -v "%PASTA_DADOS%:/app/dados" --name %NOME_CONTAINER% server-img