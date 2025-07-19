# desafio_cidade-clima

- Clone o projeto.
- Suba os containers
    docker compose up --build

- Consulta via POST (com sucesso)
    curl -X POST http://localhost:8080/cep \
        -H "Content-Type: application/json" \
        -d '{"cep": "29902555"}'
- Consulta via POST (com formato incorreto)
    curl -X POST http://localhost:8080/cep \
        -H "Content-Type: application/json" \
        -d '{"cep": "38750"}'
- Consulta via POST (CEP não existe)
    curl -X POST http://localhost:8080/cep \
        -H "Content-Type: application/json" \
        -d '{"cep": "00000000"}'

- Acesse a interface do Zipkin para observar os traces.
    http://localhost:9411/zipkin/
    Clique em RUN QUERY e expanda os spans para verificar os tempos de resposta