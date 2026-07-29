package models

import (
	"os"
	"path/filepath"
	"testing"
)

// isolate impede que os diretorios reais da maquina entrem na varredura.
func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("LLAMA_CACHE", "")
	t.Setenv("LLAMA_MODELS", "")
	t.Setenv("HOME", t.TempDir())
}

func write(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanHFLayout(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	snap := filepath.Join(root, "models--ggml-org--gemma-4-26B-A4B-it-GGUF", "snapshots", "abc123")
	write(t, filepath.Join(snap, "gemma-4-26B-A4B-it-Q4_0.gguf"), 1024)
	write(t, filepath.Join(snap, "mtp-gemma-4-26B-A4B-it-Q4_0.gguf"), 32)
	write(t, filepath.Join(snap, "mmproj-gemma-4.gguf"), 16)
	write(t, filepath.Join(root, "solto", "meu-modelo-Q8_0.gguf"), 64)

	found, warns := Scan([]string{root})
	if len(warns) != 0 {
		t.Fatalf("avisos inesperados: %v", warns)
	}
	if len(found) != 2 {
		t.Fatalf("esperava 2 modelos, veio %d: %+v", len(found), found)
	}

	var hf, local *Model
	for i := range found {
		if found[i].Source == SourceHF {
			hf = &found[i]
		} else {
			local = &found[i]
		}
	}
	if hf == nil || local == nil {
		t.Fatalf("faltou classificar: %+v", found)
	}
	if got := hf.ID(); got != "ggml-org/gemma-4-26B-A4B-it-GGUF:Q4_0" {
		t.Errorf("ID do modelo HF = %q", got)
	}
	if !hf.HasMTP() || !hf.HasMMProj() {
		t.Errorf("sidecars nao detectados: mtp=%q mmproj=%q", hf.MTPPath, hf.MMProjPath)
	}
	if hf.Size != 1024 {
		t.Errorf("tamanho = %d, sidecars nao devem entrar na conta", hf.Size)
	}
	if got := hf.Args(false); got[0] != "-hf" || got[1] != hf.ID() {
		t.Errorf("Args(-hf) = %v", got)
	}
	if got := hf.Args(true); got[0] != "-m" || got[1] != hf.Path {
		t.Errorf("Args(-m) = %v", got)
	}
	if local.Quant != "Q8_0" || local.HasMTP() {
		t.Errorf("modelo local classificado errado: %+v", local)
	}
}

func TestScanSplitShards(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	snap := filepath.Join(root, "models--org--big-GGUF", "snapshots", "s1")
	write(t, filepath.Join(snap, "big-Q4_K_M-00001-of-00003.gguf"), 100)
	write(t, filepath.Join(snap, "big-Q4_K_M-00002-of-00003.gguf"), 100)
	write(t, filepath.Join(snap, "big-Q4_K_M-00003-of-00003.gguf"), 50)

	found, _ := Scan([]string{root})
	if len(found) != 1 {
		t.Fatalf("shards deviam virar um unico modelo, veio %d", len(found))
	}
	m := found[0]
	if m.Size != 250 {
		t.Errorf("tamanho somado = %d, esperava 250", m.Size)
	}
	if m.Shards != 3 {
		t.Errorf("shards = %d, esperava 3", m.Shards)
	}
	if m.Quant != "Q4_K_M" {
		t.Errorf("quant = %q", m.Quant)
	}
}

func TestQuantOf(t *testing.T) {
	cases := map[string]string{
		"gpt-oss-20b-MXFP4":            "MXFP4",
		"gemma-4-26B-A4B-it-Q4_0":      "Q4_0",
		"embeddinggemma-300M-qat-Q4_0": "Q4_0",
		"semquant":                     "",
	}
	for in, want := range cases {
		if got := quantOf(in); got != want {
			t.Errorf("quantOf(%q) = %q, esperava %q", in, got, want)
		}
	}
}
