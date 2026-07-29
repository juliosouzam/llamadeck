# llamadeck

TUI em Go + [bubbletea](https://github.com/charmbracelet/bubbletea) para configurar e operar o `llama-server`
do llama.cpp: escolher o modelo entre os GGUF ja baixados, marcar os parametros num formulario com checkbox,
subir o servidor, acompanhar os logs ao vivo, parar e reiniciar.

```
llamadeck  ● rodando  pid 41233  up 00:03:21  http://127.0.0.1:8080
modelo  ggml-org/gemma-4-26B-A4B-it-GGUF:Q4_0  13.6 GiB  MTP  via -hf  spec=draft-mtp
 1 Modelos │ 2 Parametros │ 3 Perfis │ 4 Logs
──────────────────────────────────────────────────────────────────────────────
comando
  llama-server -hf ggml-org/gemma-4-26B-A4B-it-GGUF:Q4_0 --host 127.0.0.1
  --port 8080 --ctx-size 32768 --cache-type-k q8_0 --cache-type-v q8_0
  --n-gpu-layers 99 --spec-type draft-mtp --jinja
──────────────────────────────────────────────────────────────────────────────
▪ Contexto & KV cache
▸ [x] -c, --ctx-size                 32768            Tamanho do contexto
  [x] -ctk, --cache-type-k           q8_0             Tipo do KV (K)
  [ ] --swa-full                     -                SWA cache completo
```

## Requisitos

- Go 1.26
- `llama-server` no PATH (`brew install llama.cpp` ou build local)

## Instalacao

```bash
cd llamadeck
make install          # go install em $(go env GOPATH)/bin
# ou
go build -o llamadeck .
```

## Uso

```bash
llamadeck                      # abre a TUI
llamadeck --list-models        # lista os GGUF encontrados e sai
llamadeck --print              # imprime o comando do perfil atual e sai
llamadeck --profile gemma-mtp  # abre ja com um perfil salvo carregado
```

## De onde vem a lista de modelos

A varredura cobre, nesta ordem:

1. `$LLAMA_CACHE` - onde o llama.cpp grava os downloads do `-hf`
2. `$LLAMA_MODELS`
3. diretorios extras configurados na aba **Perfis** (tecla `D`)
4. `~/Library/Caches/llama.cpp` (macOS), `~/.cache/llama.cpp` e `~/models`

Entende o layout do cache do Hugging Face
(`models--<org>--<repo>/snapshots/<sha>/*.gguf`) e tambem GGUF soltos.
Arquivos divididos (`-00001-of-00003.gguf`) viram uma entrada so, com o tamanho somado.

Modelos no layout HF sobem por `-hf org/repo:QUANT`; GGUF soltos sobem por `-m caminho`.
A tecla `p` alterna entre os dois para o modelo selecionado.

## MTP e speculative decoding

O sidecar `mtp-*.gguf` e detectado na varredura e aparece com o badge `MTP`. Para usar:

- tecla `m` na aba **Modelos**, ou `--spec-type` = `draft-mtp` na aba **Parametros**
- o modelo precisa subir via `-hf`: com `-m` o servidor nao procura o sidecar no repo
- confirme no log: `adding speculative implementation 'draft-mtp'`
- taxas de aceite em `/metrics` (o parametro `--metrics` ja vem marcado por padrao)

## Teclas

Globais:

| tecla | acao |
|---|---|
| `tab` / `shift+tab` / `1`-`4` | trocar de aba |
| `^r` | subir o servidor (ou reiniciar, se ja estiver no ar) |
| `^x` | parar |
| `^s` | salvar a configuracao em disco |
| `q` | sair (com o servidor no ar, pergunta se para ou deixa rodando) |

Aba **Modelos**: `enter` seleciona · `p` alterna `-m`/`-hf` · `m` liga/desliga MTP ·
`n` liga/desliga `--no-mmproj` · `r` refaz a varredura · `/` filtra

Aba **Parametros**: `espaco` marca/desmarca · `enter` edita ou cicla o valor ·
`←`/`→` cicla enums e toggles · `e` edita como texto livre · `d` volta ao default ·
`X` desmarca todos os visiveis · `/` filtra (busca em flag, rotulo e ajuda)

Aba **Perfis**: `enter` carrega · `s` salva como · `o` sobrescreve · `x` apaga ·
`b` caminho do binario · `D` diretorios extras · `e` env do processo ·
`E` repassar ou nao as `LLAMA_ARG_*` do shell

Aba **Logs**: `f` follow · `g`/`G` topo/fim · `^u`/`^d` meia pagina · `c` limpa · `/` filtra

## Parametros cobertos

O catalogo em `internal/catalog` cobre o `llama-server --help` inteiro, agrupado em:
Servidor, Contexto & KV cache, GPU & memoria, Speculative decoding & MTP, Sampling,
Chat & raciocinio, Multimodal, Slots & batching, CPU & threads, Router multi-modelo,
Logs/LoRA/rede.

Cada parametro tem tipo (flag, toggle `--x`/`--no-x`, inteiro, float, texto, enum),
valor default e o texto de ajuda do proprio `llama-server`.

## Configuracao

Fica em `~/.config/llamadeck/config.json` (respeita `XDG_CONFIG_HOME`), com o perfil
atual e os perfis salvos. E gravado ao sair e com `^s`.

Por padrao as variaveis `LLAMA_ARG_*` do seu shell **nao** sao repassadas ao servidor,
para que so os parametros marcados na TUI valham. Inverta com `E` na aba Perfis.

## Desenvolvimento

```bash
make test     # go vet + go test ./...
make e2e      # sobe o llama-server de verdade com um GGUF pequeno
make fmt
```

O `make e2e` precisa do binario no PATH e de um modelo:

```bash
LLAMADECK_E2E=1 LLAMADECK_E2E_MODEL=/caminho/modelo.gguf go test ./internal/server -run RealLlama -v
```

Layout:

```
main.go                  flags de linha de comando e bootstrap
internal/catalog         catalogo de parametros do llama-server
internal/models          varredura dos GGUF no disco
internal/config          perfis e persistencia
internal/server          ciclo de vida do processo, ring buffer de logs, health check
internal/ui              a TUI (uma aba por arquivo)
```
