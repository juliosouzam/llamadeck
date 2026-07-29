package config

import (
	"strings"
	"testing"
)

func TestProfileArgs(t *testing.T) {
	p := DefaultProfile()
	p.Model = ModelRef{Repo: "ggml-org/gemma-4-26B-A4B-it-GGUF", Quant: "Q4_0"}
	p.Params["spec-type"] = ParamValue{Enabled: true, Value: "draft-mtp"}
	p.Params["mmproj-auto"] = ParamValue{Enabled: true, Value: "off"}
	p.Params["swa-full"] = ParamValue{Enabled: false, Value: ""}

	got := strings.Join(p.Args(), " ")
	for _, want := range []string{
		"-hf ggml-org/gemma-4-26B-A4B-it-GGUF:Q4_0",
		"--spec-type draft-mtp",
		"--no-mmproj",
		"--ctx-size 32768",
		"--cache-type-k q8_0",
		"--cache-type-v q8_0",
		"--n-gpu-layers 99",
		"--jinja",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("faltou %q em: %s", want, got)
		}
	}
	if strings.Contains(got, "--swa-full") {
		t.Errorf("parametro desmarcado vazou: %s", got)
	}
	if !strings.HasPrefix(got, "-hf ") {
		t.Errorf("o modelo deve vir primeiro: %s", got)
	}
}

func TestModelRefLocalPath(t *testing.T) {
	r := ModelRef{Repo: "org/repo", Quant: "Q4_0", Path: "/tmp/a.gguf", UseLocalPath: true}
	if got := strings.Join(r.Args(), " "); got != "-m /tmp/a.gguf" {
		t.Errorf("Args = %q", got)
	}
	r.UseLocalPath = false
	if got := strings.Join(r.Args(), " "); got != "-hf org/repo:Q4_0" {
		t.Errorf("Args = %q", got)
	}
}

func TestEndpoint(t *testing.T) {
	p := Profile{Params: map[string]ParamValue{}}
	if got := p.Endpoint(); got != "http://127.0.0.1:8080" {
		t.Errorf("default = %q", got)
	}
	p.Params["host"] = ParamValue{Enabled: true, Value: "0.0.0.0"}
	p.Params["port"] = ParamValue{Enabled: true, Value: "9090"}
	if got := p.Endpoint(); got != "http://127.0.0.1:9090" {
		t.Errorf("0.0.0.0 deve virar loopback no health check, veio %q", got)
	}
	p.Params["host"] = ParamValue{Enabled: true, Value: "/tmp/llama.sock"}
	if got := p.Endpoint(); got != "" {
		t.Errorf("socket unix nao tem endpoint http, veio %q", got)
	}
}

func TestShellQuoting(t *testing.T) {
	got := Shell("llama-server", []string{"--chat-template-kwargs", `{"a":"b c"}`, "-m", "/tmp/um modelo.gguf"})
	want := `llama-server --chat-template-kwargs '{"a":"b c"}' -m '/tmp/um modelo.gguf'`
	if got != want {
		t.Errorf("Shell =\n%s\nesperava\n%s", got, want)
	}
}
