// Package catalog descreve os parâmetros aceitos pelo llama-server.
//
// Cada Spec sabe como se transformar em argumentos de linha de comando, o que
// permite a TUI renderizar um formulário genérico sem conhecer flag por flag.
package catalog

import "strings"

type Kind int

const (
	// KindFlag: flag sem valor. Marcado = flag presente.
	KindFlag Kind = iota
	// KindToggle: flag com forma negativa (--jinja / --no-jinja).
	KindToggle
	KindInt
	KindFloat
	KindString
	// KindEnum: valor restrito a Choices, mas editável como texto livre.
	KindEnum
)

type Spec struct {
	ID      string
	Flag    string
	OffFlag string
	Short   string
	Label   string
	Kind    Kind
	Default string
	Choices []string
	Help    string
	Group   string
}

type Group struct {
	Name  string
	Specs []Spec
}

// Args converte o valor corrente do parâmetro nos argumentos do processo.
func (s Spec) Args(value string) []string {
	switch s.Kind {
	case KindFlag:
		return []string{s.Flag}
	case KindToggle:
		if strings.EqualFold(value, "off") || strings.EqualFold(value, "false") {
			if s.OffFlag == "" {
				return nil
			}
			return []string{s.OffFlag}
		}
		return []string{s.Flag}
	default:
		if strings.TrimSpace(value) == "" {
			return nil
		}
		return []string{s.Flag, value}
	}
}

// Display devolve a forma curta e longa da flag, para exibição.
func (s Spec) Display() string {
	if s.Short != "" {
		return s.Short + ", " + s.Flag
	}
	return s.Flag
}

// Next devolve o próximo valor no ciclo (enum e toggle).
func (s Spec) Next(value string) string {
	choices := s.Choices
	if s.Kind == KindToggle {
		choices = []string{"on", "off"}
	}
	if len(choices) == 0 {
		return value
	}
	for i, c := range choices {
		if c == value {
			return choices[(i+1)%len(choices)]
		}
	}
	return choices[0]
}

// Prev devolve o valor anterior no ciclo (enum e toggle).
func (s Spec) Prev(value string) string {
	choices := s.Choices
	if s.Kind == KindToggle {
		choices = []string{"on", "off"}
	}
	if len(choices) == 0 {
		return value
	}
	for i, c := range choices {
		if c == value {
			return choices[(i-1+len(choices))%len(choices)]
		}
	}
	return choices[len(choices)-1]
}

var kvTypes = []string{"f16", "q8_0", "q4_0", "q4_1", "q5_0", "q5_1", "iq4_nl", "bf16", "f32"}

// Groups é o catálogo completo, na ordem em que aparece na TUI.
var Groups = []Group{
	{
		Name: "Servidor",
		Specs: []Spec{
			{ID: "host", Flag: "--host", Label: "Endereço de bind", Kind: KindString, Default: "127.0.0.1", Help: "IP para escutar, ou socket unix se terminar em .sock"},
			{ID: "port", Flag: "--port", Label: "Porta", Kind: KindInt, Default: "8080", Help: "Porta TCP do servidor"},
			{ID: "alias", Flag: "--alias", Short: "-a", Label: "Alias do modelo", Kind: KindString, Help: "Nome exposto na API, separado por vírgula para vários"},
			{ID: "tags", Flag: "--tags", Label: "Tags", Kind: KindString, Help: "Tags informativas, separadas por vírgula"},
			{ID: "api-key", Flag: "--api-key", Label: "API key", Kind: KindString, Help: "Chave exigida no header Authorization"},
			{ID: "api-key-file", Flag: "--api-key-file", Label: "Arquivo de API keys", Kind: KindString, Help: "Arquivo com uma chave por linha"},
			{ID: "api-prefix", Flag: "--api-prefix", Label: "Prefixo da API", Kind: KindString, Help: "Prefixo das rotas, sem barra final"},
			{ID: "static-path", Flag: "--path", Label: "Diretório estático", Kind: KindString, Help: "Serve arquivos estáticos deste diretório"},
			{ID: "timeout", Flag: "--timeout", Short: "-to", Label: "Timeout (s)", Kind: KindInt, Default: "3600", Help: "Timeout de leitura/escrita HTTP"},
			{ID: "sse-ping-interval", Flag: "--sse-ping-interval", Label: "SSE ping (s)", Kind: KindInt, Default: "30", Help: "Intervalo de ping do SSE, -1 desativa"},
			{ID: "threads-http", Flag: "--threads-http", Label: "Threads HTTP", Kind: KindInt, Default: "-1", Help: "Threads para processar requisições HTTP"},
			{ID: "reuse-port", Flag: "--reuse-port", Label: "SO_REUSEPORT", Kind: KindFlag, Help: "Permite vários sockets na mesma porta"},
			{ID: "metrics", Flag: "--metrics", Label: "Endpoint /metrics", Kind: KindFlag, Help: "Métricas no formato Prometheus (útil para ver taxa de aceite do MTP)"},
			{ID: "props", Flag: "--props", Label: "POST /props", Kind: KindFlag, Help: "Permite alterar propriedades globais em runtime"},
			{ID: "slots-endpoint", Flag: "--slots", OffFlag: "--no-slots", Label: "Endpoint /slots", Kind: KindToggle, Default: "on", Help: "Expoe o monitoramento de slots"},
			{ID: "webui", Flag: "--webui", OffFlag: "--no-webui", Label: "Web UI", Kind: KindToggle, Default: "on", Help: "Interface web embutida"},
			{ID: "tools", Flag: "--tools", Label: "Ferramentas embutidas", Kind: KindString, Help: "read_file, file_glob_search, grep_search, exec_shell_command, write_file, edit_file, get_datetime ou all. Não use em ambiente não confiável"},
			{ID: "agent", Flag: "--agent", OffFlag: "--no-agent", Label: "Modo agente", Kind: KindToggle, Default: "off", Help: "Liga proxy CORS e todas as ferramentas. Não use em ambiente não confiável"},
			{ID: "ssl-key-file", Flag: "--ssl-key-file", Label: "Chave SSL", Kind: KindString, Help: "Chave privada PEM"},
			{ID: "ssl-cert-file", Flag: "--ssl-cert-file", Label: "Certificado SSL", Kind: KindString, Help: "Certificado PEM"},
		},
	},
	{
		Name: "Contexto & KV cache",
		Specs: []Spec{
			{ID: "ctx-size", Flag: "--ctx-size", Short: "-c", Label: "Tamanho do contexto", Kind: KindInt, Default: "0", Help: "0 usa o contexto nativo do modelo. Em 24 GB de RAM unificada, 32768 com KV q8_0 deixa folga"},
			{ID: "n-predict", Flag: "--predict", Short: "-n", Label: "Tokens a prever", Kind: KindInt, Default: "-1", Help: "-1 = infinito"},
			{ID: "batch-size", Flag: "--batch-size", Short: "-b", Label: "Batch lógico", Kind: KindInt, Default: "2048", Help: "Tamanho máximo do batch lógico"},
			{ID: "ubatch-size", Flag: "--ubatch-size", Short: "-ub", Label: "Batch físico", Kind: KindInt, Default: "512", Help: "Tamanho máximo do micro batch"},
			{ID: "keep", Flag: "--keep", Label: "Tokens preservados", Kind: KindInt, Default: "0", Help: "Tokens do prompt inicial mantidos, -1 = todos"},
			{ID: "cache-type-k", Flag: "--cache-type-k", Short: "-ctk", Label: "Tipo do KV (K)", Kind: KindEnum, Default: "f16", Choices: kvTypes, Help: "q8_0 corta pela metade a memória do cache de chaves"},
			{ID: "cache-type-v", Flag: "--cache-type-v", Short: "-ctv", Label: "Tipo do KV (V)", Kind: KindEnum, Default: "f16", Choices: kvTypes, Help: "q8_0 corta pela metade a memória do cache de valores"},
			{ID: "swa-full", Flag: "--swa-full", Label: "SWA cache completo", Kind: KindFlag, Help: "Usa cache SWA de tamanho total, gasta bem mais memória"},
			{ID: "context-shift", Flag: "--context-shift", OffFlag: "--no-context-shift", Label: "Context shift", Kind: KindToggle, Default: "off", Help: "Descarta o início do contexto ao estourar em vez de parar"},
			{ID: "ctx-checkpoints", Flag: "--ctx-checkpoints", Short: "-ctxcp", Label: "Checkpoints por slot", Kind: KindInt, Default: "32", Help: "Número máximo de checkpoints de contexto por slot"},
			{ID: "checkpoint-min-step", Flag: "--checkpoint-min-step", Short: "-cms", Label: "Espaço entre checkpoints", Kind: KindInt, Default: "8192", Help: "Espaçamento mínimo em tokens, 0 = sem mínimo"},
			{ID: "cache-ram", Flag: "--cache-ram", Short: "-cram", Label: "Cache em RAM (MiB)", Kind: KindInt, Default: "8192", Help: "-1 = sem limite, 0 = desativa"},
			{ID: "kv-unified", Flag: "--kv-unified", OffFlag: "--no-kv-unified", Label: "KV unificado", Kind: KindToggle, Default: "on", Help: "Buffer KV único compartilhado entre as sequencias"},
			{ID: "cache-prompt", Flag: "--cache-prompt", OffFlag: "--no-cache-prompt", Label: "Cache de prompt", Kind: KindToggle, Default: "on", Help: "Reaproveita prefixos de prompt entre requisições"},
			{ID: "cache-reuse", Flag: "--cache-reuse", Label: "Reuso via KV shift", Kind: KindInt, Default: "0", Help: "Tamanho mínimo do chunk reaproveitado via KV shifting"},
			{ID: "cache-idle-slots", Flag: "--cache-idle-slots", OffFlag: "--no-cache-idle-slots", Label: "Cachear slots ociosos", Kind: KindToggle, Default: "on", Help: "Salva slots ociosos no cache de prompt"},
			{ID: "kv-offload", Flag: "--kv-offload", OffFlag: "--no-kv-offload", Label: "Offload do KV", Kind: KindToggle, Default: "on", Help: "Mantem o KV cache na GPU"},
		},
	},
	{
		Name: "GPU & memória",
		Specs: []Spec{
			{ID: "n-gpu-layers", Flag: "--n-gpu-layers", Short: "-ngl", Label: "Camadas na GPU", Kind: KindString, Default: "auto", Help: "Número exato, 'auto' ou 'all'. 99 força offload total no Metal"},
			{ID: "device", Flag: "--device", Short: "-dev", Label: "Dispositivos", Kind: KindString, Help: "Lista separada por vírgula, 'none' desliga o offload"},
			{ID: "split-mode", Flag: "--split-mode", Short: "-sm", Label: "Modo de split", Kind: KindEnum, Default: "layer", Choices: []string{"none", "layer", "row", "tensor"}, Help: "Como dividir o modelo entre varias GPUs"},
			{ID: "tensor-split", Flag: "--tensor-split", Short: "-ts", Label: "Proporção por GPU", Kind: KindString, Help: "Frações separadas por vírgula, ex: 3,1"},
			{ID: "main-gpu", Flag: "--main-gpu", Short: "-mg", Label: "GPU principal", Kind: KindInt, Default: "0", Help: "Índice da GPU principal"},
			{ID: "fit", Flag: "--fit", Short: "-fit", Label: "Auto ajuste de memória", Kind: KindEnum, Default: "on", Choices: []string{"on", "off"}, Help: "Ajusta argumentos não definidos para caber na memória do device"},
			{ID: "fit-target", Flag: "--fit-target", Short: "-fitt", Label: "Margem do fit (MiB)", Kind: KindString, Default: "1024", Help: "Margem por device, separada por vírgula"},
			{ID: "fit-ctx", Flag: "--fit-ctx", Short: "-fitc", Label: "Contexto mínimo do fit", Kind: KindInt, Default: "4096", Help: "Menor contexto que o --fit pode escolher"},
			{ID: "flash-attn", Flag: "--flash-attn", Short: "-fa", Label: "Flash attention", Kind: KindEnum, Default: "auto", Choices: []string{"auto", "on", "off"}, Help: "Reduz memória e acelera a atenção quando suportado"},
			{ID: "cpu-moe", Flag: "--cpu-moe", Short: "-cmoe", Label: "MoE na CPU", Kind: KindFlag, Help: "Mantem todos os pesos de Mixture of Experts na CPU"},
			{ID: "n-cpu-moe", Flag: "--n-cpu-moe", Short: "-ncmoe", Label: "Camadas MoE na CPU", Kind: KindInt, Help: "Mantem o MoE das N primeiras camadas na CPU"},
			{ID: "mlock", Flag: "--mlock", Label: "mlock", Kind: KindFlag, Help: "Trava o modelo na RAM, sem swap nem compressão"},
			{ID: "mmap", Flag: "--mmap", OffFlag: "--no-mmap", Label: "mmap", Kind: KindToggle, Default: "on", Help: "Mapeia o modelo em memória, carga mais rapida"},
			{ID: "direct-io", Flag: "--direct-io", OffFlag: "--no-direct-io", Label: "Direct IO", Kind: KindToggle, Default: "off", Help: "Usa DirectIO quando disponível"},
			{ID: "no-host", Flag: "--no-host", Label: "Ignorar host buffer", Kind: KindFlag, Help: "Permite usar buffers extras no lugar do host buffer"},
			{ID: "numa", Flag: "--numa", Label: "NUMA", Kind: KindEnum, Choices: []string{"distribute", "isolate", "numactl"}, Help: "Otimizações para sistemas NUMA"},
			{ID: "override-tensor", Flag: "--override-tensor", Short: "-ot", Label: "Override de tensor", Kind: KindString, Help: "padrão=buffer_type, separado por vírgula"},
			{ID: "override-kv", Flag: "--override-kv", Label: "Override de metadado", Kind: KindString, Help: "CHAVE=TIPO:VALOR, separado por vírgula"},
			{ID: "check-tensors", Flag: "--check-tensors", Label: "Validar tensores", Kind: KindFlag, Help: "Checa os dados do modelo por valores inválidos"},
			{ID: "op-offload", Flag: "--op-offload", OffFlag: "--no-op-offload", Label: "Offload de operações", Kind: KindToggle, Default: "on", Help: "Offload de operações de tensor do host para o device"},
		},
	},
	{
		Name: "Speculative decoding & MTP",
		Specs: []Spec{
			{ID: "spec-type", Flag: "--spec-type", Label: "Tipo de especulação", Kind: KindEnum, Default: "none", Choices: []string{"none", "draft-mtp", "draft-simple", "draft-eagle3", "draft-dflash", "ngram-simple", "ngram-map-k", "ngram-map-k4v", "ngram-mod", "ngram-cache"}, Help: "draft-mtp liga o head MTP. Obrigatório: sem ele o server nem procura o sidecar mtp-*.gguf no repo"},
			{ID: "spec-draft-model", Flag: "--spec-draft-model", Short: "-md", Label: "Modelo draft", Kind: KindString, Help: "Caminho do GGUF do modelo rascunho"},
			{ID: "spec-draft-hf", Flag: "--spec-draft-hf", Short: "-hfd", Label: "Repo HF do draft", Kind: KindString, Help: "org/repo[:quant] do modelo rascunho"},
			{ID: "spec-draft-n-max", Flag: "--spec-draft-n-max", Label: "Draft n max", Kind: KindInt, Default: "3", Help: "Tokens rascunhados por passo"},
			{ID: "spec-draft-n-min", Flag: "--spec-draft-n-min", Label: "Draft n min", Kind: KindInt, Default: "0", Help: "Mínimo de tokens rascunhados"},
			{ID: "spec-draft-p-split", Flag: "--spec-draft-p-split", Label: "Probabilidade de split", Kind: KindFloat, Default: "0.10", Help: "Probabilidade de split na arvore de rascunho"},
			{ID: "spec-draft-p-min", Flag: "--spec-draft-p-min", Label: "Probabilidade mínima", Kind: KindFloat, Default: "0.00", Help: "Probabilidade mínima para aceitar o rascunho"},
			{ID: "spec-draft-ngl", Flag: "--spec-draft-ngl", Short: "-ngld", Label: "Camadas do draft na GPU", Kind: KindString, Default: "auto", Help: "Número exato, 'auto' ou 'all'"},
			{ID: "spec-draft-device", Flag: "--spec-draft-device", Short: "-devd", Label: "Dispositivos do draft", Kind: KindString, Help: "Lista separada por vírgula"},
			{ID: "spec-draft-type-k", Flag: "--spec-draft-type-k", Short: "-ctkd", Label: "KV do draft (K)", Kind: KindEnum, Default: "f16", Choices: kvTypes, Help: "Tipo do cache de chaves do modelo rascunho"},
			{ID: "spec-draft-type-v", Flag: "--spec-draft-type-v", Short: "-ctvd", Label: "KV do draft (V)", Kind: KindEnum, Default: "f16", Choices: kvTypes, Help: "Tipo do cache de valores do modelo rascunho"},
			{ID: "spec-draft-cpu-moe", Flag: "--spec-draft-cpu-moe", Short: "-cmoed", Label: "MoE do draft na CPU", Kind: KindFlag, Help: "Mantem o MoE do rascunho na CPU"},
			{ID: "spec-draft-backend-sampling", Flag: "--spec-draft-backend-sampling", OffFlag: "--no-spec-draft-backend-sampling", Label: "Sampling do draft na GPU", Kind: KindToggle, Default: "on", Help: "Faz o sampling do rascunho no backend"},
			{ID: "spec-ngram-mod-n-min", Flag: "--spec-ngram-mod-n-min", Label: "ngram-mod n min", Kind: KindInt, Default: "48", Help: "Mínimo de tokens do ngram-mod"},
			{ID: "spec-ngram-mod-n-max", Flag: "--spec-ngram-mod-n-max", Label: "ngram-mod n max", Kind: KindInt, Default: "64", Help: "Máximo de tokens do ngram-mod"},
			{ID: "spec-ngram-mod-n-match", Flag: "--spec-ngram-mod-n-match", Label: "ngram-mod match", Kind: KindInt, Default: "24", Help: "Comprimento do lookup do ngram-mod"},
			{ID: "spec-ngram-simple-size-n", Flag: "--spec-ngram-simple-size-n", Label: "ngram-simple N", Kind: KindInt, Default: "12", Help: "Comprimento do n-grama de lookup"},
			{ID: "spec-ngram-simple-size-m", Flag: "--spec-ngram-simple-size-m", Label: "ngram-simple M", Kind: KindInt, Default: "48", Help: "Comprimento do m-grama rascunhado"},
			{ID: "spec-ngram-simple-min-hits", Flag: "--spec-ngram-simple-min-hits", Label: "ngram-simple hits", Kind: KindInt, Default: "1", Help: "Hits mínimos para usar o rascunho"},
			{ID: "lookup-cache-static", Flag: "--lookup-cache-static", Short: "-lcs", Label: "Lookup cache estático", Kind: KindString, Help: "Arquivo de cache de lookup não atualizado"},
			{ID: "lookup-cache-dynamic", Flag: "--lookup-cache-dynamic", Short: "-lcd", Label: "Lookup cache dinâmico", Kind: KindString, Help: "Arquivo de cache de lookup atualizado durante a geração"},
		},
	},
	{
		Name: "Sampling",
		Specs: []Spec{
			{ID: "samplers", Flag: "--samplers", Label: "Ordem dos samplers", Kind: KindString, Default: "penalties;dry;top_n_sigma;top_k;typ_p;top_p;min_p;xtc;temperature", Help: "Separados por ponto e vírgula"},
			{ID: "seed", Flag: "--seed", Short: "-s", Label: "Seed", Kind: KindInt, Default: "-1", Help: "-1 usa seed aleatoria"},
			{ID: "temp", Flag: "--temp", Label: "Temperatura", Kind: KindFloat, Default: "0.80", Help: "Temperatura da amostragem"},
			{ID: "top-k", Flag: "--top-k", Label: "Top-k", Kind: KindInt, Default: "40", Help: "0 desativa"},
			{ID: "top-p", Flag: "--top-p", Label: "Top-p", Kind: KindFloat, Default: "0.95", Help: "1.0 desativa"},
			{ID: "min-p", Flag: "--min-p", Label: "Min-p", Kind: KindFloat, Default: "0.05", Help: "0.0 desativa"},
			{ID: "top-n-sigma", Flag: "--top-n-sigma", Label: "Top-n-sigma", Kind: KindFloat, Default: "-1.00", Help: "-1.0 desativa"},
			{ID: "typical", Flag: "--typical", Label: "Typical-p", Kind: KindFloat, Default: "1.00", Help: "1.0 desativa"},
			{ID: "repeat-last-n", Flag: "--repeat-last-n", Label: "Janela de repetição", Kind: KindInt, Default: "64", Help: "0 desativa, -1 usa o contexto inteiro"},
			{ID: "repeat-penalty", Flag: "--repeat-penalty", Label: "Penalidade de repetição", Kind: KindFloat, Default: "1.00", Help: "1.0 desativa"},
			{ID: "presence-penalty", Flag: "--presence-penalty", Label: "Presence penalty", Kind: KindFloat, Default: "0.00", Help: "Penalidade alpha de presenca"},
			{ID: "frequency-penalty", Flag: "--frequency-penalty", Label: "Frequency penalty", Kind: KindFloat, Default: "0.00", Help: "Penalidade alpha de frequência"},
			{ID: "dry-multiplier", Flag: "--dry-multiplier", Label: "DRY multiplier", Kind: KindFloat, Default: "0.00", Help: "0.0 desativa o DRY"},
			{ID: "dry-base", Flag: "--dry-base", Label: "DRY base", Kind: KindFloat, Default: "1.75", Help: "Base do DRY"},
			{ID: "dry-allowed-length", Flag: "--dry-allowed-length", Label: "DRY comprimento", Kind: KindInt, Default: "2", Help: "Comprimento permitido antes da penalidade"},
			{ID: "dry-penalty-last-n", Flag: "--dry-penalty-last-n", Label: "DRY janela", Kind: KindInt, Default: "-1", Help: "0 desativa, -1 usa o contexto"},
			{ID: "xtc-probability", Flag: "--xtc-probability", Label: "XTC probabilidade", Kind: KindFloat, Default: "0.00", Help: "0.0 desativa"},
			{ID: "xtc-threshold", Flag: "--xtc-threshold", Label: "XTC limiar", Kind: KindFloat, Default: "0.10", Help: "1.0 desativa"},
			{ID: "dynatemp-range", Flag: "--dynatemp-range", Label: "Dynatemp faixa", Kind: KindFloat, Default: "0.00", Help: "0.0 desativa a temperatura dinâmica"},
			{ID: "dynatemp-exp", Flag: "--dynatemp-exp", Label: "Dynatemp expoente", Kind: KindFloat, Default: "1.00", Help: "Expoente da temperatura dinâmica"},
			{ID: "mirostat", Flag: "--mirostat", Label: "Mirostat", Kind: KindEnum, Default: "0", Choices: []string{"0", "1", "2"}, Help: "Ignora top-k, top-p e typical quando ativo"},
			{ID: "mirostat-lr", Flag: "--mirostat-lr", Label: "Mirostat eta", Kind: KindFloat, Default: "0.10", Help: "Taxa de aprendizado do Mirostat"},
			{ID: "mirostat-ent", Flag: "--mirostat-ent", Label: "Mirostat tau", Kind: KindFloat, Default: "5.00", Help: "Entropia alvo do Mirostat"},
			{ID: "adaptive-target", Flag: "--adaptive-target", Label: "Adaptive-p alvo", Kind: KindFloat, Default: "-1.00", Help: "Negativo desativa"},
			{ID: "adaptive-decay", Flag: "--adaptive-decay", Label: "Adaptive-p decay", Kind: KindFloat, Default: "0.90", Help: "Valores menores reagem mais rapido"},
			{ID: "ignore-eos", Flag: "--ignore-eos", Label: "Ignorar EOS", Kind: KindFlag, Help: "Continua gerando após o fim de sequência"},
			{ID: "grammar-file", Flag: "--grammar-file", Label: "Arquivo de gramática", Kind: KindString, Help: "Gramática BNF para restringir a geração"},
			{ID: "json-schema-file", Flag: "--json-schema-file", Short: "-jf", Label: "Arquivo de JSON schema", Kind: KindString, Help: "Restringe a saída a um JSON schema"},
			{ID: "backend-sampling", Flag: "--backend-sampling", Short: "-bs", Label: "Sampling no backend", Kind: KindFlag, Help: "Experimental: faz o sampling na GPU"},
		},
	},
	{
		Name: "Chat & raciocínio",
		Specs: []Spec{
			{ID: "jinja", Flag: "--jinja", OffFlag: "--no-jinja", Label: "Motor Jinja", Kind: KindToggle, Default: "on", Help: "Usa o template de chat do modelo via Jinja"},
			{ID: "chat-template", Flag: "--chat-template", Label: "Template de chat", Kind: KindString, Help: "Template embutido (chatml, gemma, llama3, gpt-oss, ...) ou Jinja completo"},
			{ID: "chat-template-file", Flag: "--chat-template-file", Label: "Arquivo de template", Kind: KindString, Help: "Arquivo com o template Jinja"},
			{ID: "chat-template-kwargs", Flag: "--chat-template-kwargs", Label: "Kwargs do template", Kind: KindString, Help: "Objeto JSON com parâmetros extras do template"},
			{ID: "reasoning", Flag: "--reasoning", Short: "-rea", Label: "Raciocínio", Kind: KindEnum, Default: "auto", Choices: []string{"auto", "on", "off"}, Help: "Liga o modo de pensamento no chat"},
			{ID: "reasoning-format", Flag: "--reasoning-format", Label: "Formato do raciocínio", Kind: KindEnum, Default: "auto", Choices: []string{"auto", "none", "deepseek", "deepseek-legacy"}, Help: "Onde as tags de pensamento são devolvidas"},
			{ID: "reasoning-budget", Flag: "--reasoning-budget", Label: "Orcamento de raciocínio", Kind: KindInt, Default: "-1", Help: "-1 sem limite, 0 encerra na hora, N > 0 limita os tokens"},
			{ID: "reasoning-budget-message", Flag: "--reasoning-budget-message", Label: "Mensagem do orcamento", Kind: KindString, Help: "Injetada antes da tag de fim quando o orcamento acaba"},
			{ID: "reasoning-preserve", Flag: "--reasoning-preserve", OffFlag: "--no-reasoning-preserve", Label: "Preservar raciocínio", Kind: KindToggle, Default: "off", Help: "Mantem o traco de raciocínio em todo o histórico"},
		},
	},
	{
		Name: "Multimodal",
		Specs: []Spec{
			{ID: "mmproj-auto", Flag: "--mmproj-auto", OffFlag: "--no-mmproj", Label: "Baixar mmproj", Kind: KindToggle, Default: "on", Help: "Desligue para pular o projetor multimodal e economizar memória em modelos de visão"},
			{ID: "mmproj", Flag: "--mmproj", Short: "-mm", Label: "Arquivo mmproj", Kind: KindString, Help: "Caminho do projetor multimodal"},
			{ID: "mmproj-url", Flag: "--mmproj-url", Label: "URL do mmproj", Kind: KindString, Help: "URL do projetor multimodal"},
			{ID: "mmproj-offload", Flag: "--mmproj-offload", OffFlag: "--no-mmproj-offload", Label: "mmproj na GPU", Kind: KindToggle, Default: "on", Help: "Offload do projetor para a GPU"},
			{ID: "image-min-tokens", Flag: "--image-min-tokens", Label: "Tokens mínimos por imagem", Kind: KindInt, Help: "Somente para modelos de resolução dinâmica"},
			{ID: "image-max-tokens", Flag: "--image-max-tokens", Label: "Tokens máximos por imagem", Kind: KindInt, Help: "Somente para modelos de resolução dinâmica"},
			{ID: "mtmd-batch-max-tokens", Flag: "--mtmd-batch-max-tokens", Label: "Batch de tokens de imagem", Kind: KindInt, Default: "1024", Help: "Máximo de tokens de imagem por batch"},
			{ID: "media-path", Flag: "--media-path", Label: "Diretório de mídia", Kind: KindString, Help: "Permite file:// relativo a este diretório"},
		},
	},
	{
		Name: "Slots & batching",
		Specs: []Spec{
			{ID: "parallel", Flag: "--parallel", Short: "-np", Label: "Slots paralelos", Kind: KindInt, Default: "-1", Help: "-1 escolhe automaticamente"},
			{ID: "cont-batching", Flag: "--cont-batching", OffFlag: "--no-cont-batching", Label: "Batching continuo", Kind: KindToggle, Default: "on", Help: "Batching dinâmico entre requisições"},
			{ID: "slot-save-path", Flag: "--slot-save-path", Label: "Salvar KV dos slots", Kind: KindString, Help: "Diretório para persistir o KV cache dos slots"},
			{ID: "pooling", Flag: "--pooling", Label: "Pooling", Kind: KindEnum, Choices: []string{"none", "mean", "cls", "last", "rank"}, Help: "Tipo de pooling para embeddings"},
			{ID: "embedding", Flag: "--embeddings", Label: "Modo embeddings", Kind: KindFlag, Help: "Restringe o servidor a embeddings"},
			{ID: "rerank", Flag: "--reranking", Label: "Modo rerank", Kind: KindFlag, Help: "Habilita o endpoint de reranking"},
			{ID: "embd-normalize", Flag: "--embd-normalize", Label: "Normalização do embedding", Kind: KindInt, Default: "2", Help: "-1 nenhuma, 0 max abs, 1 taxicab, 2 euclidiana"},
			{ID: "warmup", Flag: "--warmup", OffFlag: "--no-warmup", Label: "Warmup", Kind: KindToggle, Default: "on", Help: "Execução vazia no start para aquecer o modelo"},
			{ID: "spm-infill", Flag: "--spm-infill", Label: "SPM infill", Kind: KindFlag, Help: "Usa o padrão suffix/prefix/middle no infill"},
			{ID: "special", Flag: "--special", Short: "-sp", Label: "Tokens especiais", Kind: KindFlag, Help: "Emite tokens especiais na saída"},
		},
	},
	{
		Name: "CPU & threads",
		Specs: []Spec{
			{ID: "threads", Flag: "--threads", Short: "-t", Label: "Threads", Kind: KindInt, Default: "-1", Help: "Threads na geração, -1 detecta"},
			{ID: "threads-batch", Flag: "--threads-batch", Short: "-tb", Label: "Threads de batch", Kind: KindInt, Help: "Threads no processamento do prompt"},
			{ID: "cpu-mask", Flag: "--cpu-mask", Short: "-C", Label: "Mascara de CPU", Kind: KindString, Help: "Mascara hexadecimal de afinidade"},
			{ID: "cpu-range", Flag: "--cpu-range", Short: "-Cr", Label: "Faixa de CPU", Kind: KindString, Help: "Faixa lo-hi de afinidade"},
			{ID: "cpu-strict", Flag: "--cpu-strict", Label: "Afinidade estrita", Kind: KindEnum, Default: "0", Choices: []string{"0", "1"}, Help: "Posicionamento estrito de CPU"},
			{ID: "prio", Flag: "--prio", Label: "Prioridade", Kind: KindEnum, Default: "0", Choices: []string{"-1", "0", "1", "2", "3"}, Help: "low(-1), normal(0), medium(1), high(2), realtime(3)"},
			{ID: "poll", Flag: "--poll", Label: "Polling", Kind: KindInt, Default: "50", Help: "0 desliga o polling, até 100"},
		},
	},
	{
		Name: "Router multi-modelo",
		Specs: []Spec{
			{ID: "models-dir", Flag: "--models-dir", Label: "Diretório de modelos", Kind: KindString, Help: "Liga o modo router servindo os modelos deste diretório"},
			{ID: "models-preset", Flag: "--models-preset", Label: "Presets do router", Kind: KindString, Help: "Arquivo INI com presets de modelo"},
			{ID: "models-max", Flag: "--models-max", Label: "Modelos simultaneos", Kind: KindInt, Default: "4", Help: "0 = ilimitado"},
			{ID: "models-autoload", Flag: "--models-autoload", OffFlag: "--no-models-autoload", Label: "Autoload no router", Kind: KindToggle, Default: "on", Help: "Carrega modelos sob demanda"},
		},
	},
	{
		Name: "Logs, LoRA & rede",
		Specs: []Spec{
			{ID: "verbosity", Flag: "--verbosity", Short: "-lv", Label: "Verbosidade", Kind: KindEnum, Default: "3", Choices: []string{"0", "1", "2", "3", "4", "5"}, Help: "0 genérico, 1 erro, 2 aviso, 3 info, 4 trace, 5 debug"},
			{ID: "log-file", Flag: "--log-file", Label: "Arquivo de log", Kind: KindString, Help: "Grava os logs também em arquivo"},
			{ID: "log-timestamps", Flag: "--log-timestamps", OffFlag: "--no-log-timestamps", Label: "Timestamps no log", Kind: KindToggle, Default: "off", Help: "Prefixa cada linha com o horário"},
			{ID: "log-prefix", Flag: "--log-prefix", OffFlag: "--no-log-prefix", Label: "Prefixo no log", Kind: KindToggle, Default: "off", Help: "Prefixa cada linha com o nível"},
			{ID: "log-colors", Flag: "--log-colors", Label: "Cores no log", Kind: KindEnum, Default: "auto", Choices: []string{"auto", "on", "off"}, Help: "A TUI captura via pipe, então 'auto' já sai sem cor"},
			{ID: "offline", Flag: "--offline", Label: "Modo offline", Kind: KindFlag, Help: "Usa somente o cache local, sem acesso a rede"},
			{ID: "hf-token", Flag: "--hf-token", Short: "-hft", Label: "Token do Hugging Face", Kind: KindString, Help: "Necessário para repos gated. Também lido de HF_TOKEN"},
			{ID: "lora", Flag: "--lora", Label: "Adaptador LoRA", Kind: KindString, Help: "Caminhos separados por vírgula"},
			{ID: "lora-scaled", Flag: "--lora-scaled", Label: "LoRA com escala", Kind: KindString, Help: "ARQUIVO:ESCALA, separado por vírgula"},
			{ID: "control-vector", Flag: "--control-vector", Label: "Control vector", Kind: KindString, Help: "Caminhos separados por vírgula"},
			{ID: "control-vector-scaled", Flag: "--control-vector-scaled", Label: "Control vector com escala", Kind: KindString, Help: "ARQUIVO:ESCALA, separado por vírgula"},
		},
	},
}

var index = func() map[string]Spec {
	m := make(map[string]Spec)
	for _, g := range Groups {
		for _, s := range g.Specs {
			s.Group = g.Name
			m[s.ID] = s
		}
	}
	return m
}()

// Lookup devolve a Spec de um ID.
func Lookup(id string) (Spec, bool) {
	s, ok := index[id]
	return s, ok
}

// All devolve todas as Specs na ordem do catálogo.
func All() []Spec {
	out := make([]Spec, 0, len(index))
	for _, g := range Groups {
		for _, s := range g.Specs {
			s.Group = g.Name
			out = append(out, s)
		}
	}
	return out
}
