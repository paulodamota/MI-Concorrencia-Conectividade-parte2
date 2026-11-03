:: attpass.bat
echo Inicializando container do passageiro...

set NUM_SERVIDOR=%1

if "%NUM_SERVIDOR%"=="1" (
    set ENDERECO="192.168.15.111:2800"
    set NOME_CONTAINER=pass1
) else if "%NUM_SERVIDOR%"=="2" (
   set ENDERECO="192.168.15.111:2900"
   set NOME_CONTAINER=pass2
) else if "%NUM_SERVIDOR%"=="3" (
    set ENDERECO="192.168.15.111:3100"
    set NOME_CONTAINER=pass3
) else (
    :: Porta padrão caso seja outro número
    set ENDERECO="192.168.15.111:2800"
)

docker build -t pass-img -f Dockerfile.passageiro .
docker stop %NOME_CONTAINER%
docker rm %NOME_CONTAINER%
docker run -it -e endereco=%ENDERECO% --name %NOME_CONTAINER% pass-img