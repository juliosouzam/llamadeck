package server

import (
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestRealLlamaServer sobe o llama-server de verdade. Precisa do binário no PATH
// e de um GGUF pequeno:
//
//	LLAMADECK_E2E=1 LLAMADECK_E2E_MODEL=/caminho/modelo.gguf go test ./internal/server -run RealLlama -v
func TestRealLlamaServer(t *testing.T) {
	if os.Getenv("LLAMADECK_E2E") == "" {
		t.Skip("defina LLAMADECK_E2E=1 para rodar contra o llama-server real")
	}
	bin, err := exec.LookPath("llama-server")
	if err != nil {
		t.Skipf("llama-server não encontrado: %v", err)
	}
	model := os.Getenv("LLAMADECK_E2E_MODEL")
	if model == "" {
		t.Skip("defina LLAMADECK_E2E_MODEL com o caminho de um GGUF pequeno")
	}

	const endpoint = "http://127.0.0.1:18099"
	m := New(4096)
	spec := StartSpec{
		Bin:      bin,
		Args:     []string{"-m", model, "--host", "127.0.0.1", "--port", "18099", "-c", "512", "--no-warmup"},
		Env:      os.Environ(),
		Endpoint: endpoint,
	}
	if err := m.Start(spec); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.StopAndWait(10 * time.Second) })

	st := waitFor(t, m, StatusRunning, 90*time.Second)
	if st.PID == 0 {
		t.Fatal("pid não registrado")
	}

	resp, err := http.Get(endpoint + "/health")
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health = %d", resp.StatusCode)
	}

	lines, _ := m.Since(0)
	var joined strings.Builder
	for _, l := range lines {
		joined.WriteString(l.Text + "\n")
	}
	if !strings.Contains(joined.String(), "loading model") {
		t.Errorf("logs do carregamento não foram capturados:\n%s", joined.String())
	}

	if err := m.Stop(10 * time.Second); err != nil {
		t.Fatal(err)
	}
	waitFor(t, m, StatusStopped, 15*time.Second)

	if _, err := http.Get(endpoint + "/health"); err == nil {
		t.Error("a porta continua atendendo depois do stop")
	}
}
